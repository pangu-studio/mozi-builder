package release

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Phase-4 acceptance: the real adapters and controller against the dedicated
// Compose stack (platform/deploy/release). Requires:
//
//	MOZI_INTEGRATION_ENV (dual databases) and MOZI_RELEASE_ACC=1, stack up via
//	make v2-acc-up
const (
	accGateway  = "http://127.0.0.1:19081"
	accAdmin    = "http://127.0.0.1:19180"
	accEtcd     = "127.0.0.1:12379"
	accAdminKey = "mozi-v2-release-acc-only"
	accRegistry = "mozi-v2-release-acc-registry-1"
)

func dockerAcc(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func dockerOut(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("docker %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// containerIP resolves a container's address on the acceptance network. The
// controller registers discovered instance addresses, not service names.
func containerIP(t *testing.T, name string) string {
	t.Helper()
	return dockerOut(t, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", name)
}

func getBody(t *testing.T, url string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, strings.TrimSpace(string(data))
}

func waitGateway(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status, body := getBody(t, accGateway+"/acc/")
		if status == 200 && strings.Contains(body, want) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("gateway did not settle on %q", want)
}

func TestReleaseAcceptance(t *testing.T) {
	if os.Getenv("MOZI_RELEASE_ACC") != "1" {
		t.Skip("MOZI_RELEASE_ACC=1 and the acceptance stack (make v2-acc-up) required")
	}
	db := isolatedPlatformDB(t)
	ctx := context.Background()
	echo1 := containerIP(t, "mozi-v2-release-acc-echo1-1") + ":80"
	echo2 := containerIP(t, "mozi-v2-release-acc-echo2-1") + ":80"
	etcd := EtcdDiscovery{Endpoints: []string{accEtcd}}
	apisix := ApisixAdmin{BaseURL: accAdmin, Key: accAdminKey}
	c := &Controller{DB: db, Etcd: etcd, Apisix: apisix, LeaseTTL: 100 * time.Millisecond}
	project := createProject(t, db)

	newOp := func(kind, key string, d Desired) string {
		raw, _ := json.Marshal(d)
		id, _, err := CreateOperation(ctx, db, OperationRow{ID: rand.Text(), ProjectID: project, Resource: "acc/echo", Kind: kind, Desired: raw}, key, "acceptance")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	run := func(id string) {
		t.Helper()
		worked, err := c.RunOnce(ctx)
		if err != nil || !worked {
			t.Fatalf("run once: %v %v", worked, err)
		}
		if state, _ := opState(t, db, id); state != Ready {
			t.Fatalf("op %s state: %s", id, state)
		}
	}

	// --- Scenario 1: scale 1 → 2 → 1 behind the gateway ---
	deploy := newOp("deploy", "acc-deploy", Desired{
		RPCService: "acc.rpc", RPCNodes: []string{echo1},
		HTTPRoute: "acc-route", HTTPURI: "/acc/*", HTTPNodes: map[string]int{echo1: 1},
	})
	run(deploy)
	waitGateway(t, "echo1")
	if nodes, err := etcd.ReadRPC(ctx, "acc.rpc"); err != nil || len(nodes) != 1 || nodes[0] != echo1 {
		t.Fatalf("rpc registry: %v %v", nodes, err)
	}

	scale2 := newOp("scale", "acc-scale-2", Desired{HTTPRoute: "acc-route", HTTPURI: "/acc/*", HTTPNodes: map[string]int{echo1: 1, echo2: 1}})
	run(scale2)
	seen := map[string]bool{}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && !(seen["echo1"] && seen["echo2"]) {
		status, body := getBody(t, accGateway+"/acc/")
		if status == 200 {
			for _, name := range []string{"echo1", "echo2"} {
				if strings.Contains(body, name) {
					seen[name] = true
				}
			}
		}
	}
	if !seen["echo1"] || !seen["echo2"] {
		t.Fatalf("expected both instances behind the gateway: %v", seen)
	}

	scale1 := newOp("scale", "acc-scale-1", Desired{HTTPRoute: "acc-route", HTTPURI: "/acc/*", HTTPNodes: map[string]int{echo1: 1}})
	run(scale1)
	waitGateway(t, "echo1")

	// --- Scenario 2: discovery registry outage keeps serving, no drift ---
	dockerAcc(t, "stop", accRegistry)
	if status, body := getBody(t, accGateway+"/acc/"); status != 200 || !strings.Contains(body, "echo1") {
		t.Fatalf("gateway must keep serving with cached config: %d %q", status, body)
	}
	row := &OperationRow{ID: scale1, Kind: "scale", State: Ready}
	row.Desired, _ = json.Marshal(Desired{HTTPRoute: "acc-route", HTTPURI: "/acc/*", HTTPNodes: map[string]int{echo1: 1}})
	if err := c.DriftCheck(ctx, row); err != nil {
		t.Fatal(err)
	}
	if state, _ := opState(t, db, scale1); state != Ready {
		t.Fatal("unreachable registry must not be marked as drift")
	}
	dockerAcc(t, "start", accRegistry)
	deadline = time.Now().Add(30 * time.Second)
	var readErr error
	for {
		_, readErr = etcd.ReadRPC(ctx, "acc.rpc")
		if readErr == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if readErr != nil {
		t.Fatalf("registry did not recover: %v", readErr)
	}
	if err := c.DriftCheck(ctx, row); err != nil {
		t.Fatal(err)
	}
	if state, _ := opState(t, db, scale1); state != Ready {
		t.Fatal("recovered registry must read back as Ready")
	}

	// --- Scenario 3: controller crash mid-apply is reclaimed and finished ---
	crash := newOp("scale", "acc-crash", Desired{HTTPRoute: "acc-route", HTTPURI: "/acc/*", HTTPNodes: map[string]int{echo2: 1}})
	claimed, err := c.ClaimNext(ctx)
	if err != nil || claimed == nil || claimed.ID != crash {
		t.Fatalf("claim: %v %v", claimed, err)
	}
	// Simulate the crash: the claimed operation is never executed, and its
	// lease has long expired (aged in the DB to avoid clock skew).
	c2 := &Controller{DB: db, Etcd: etcd, Apisix: apisix, LeaseTTL: time.Millisecond}
	if _, err := db.ExecContext(ctx, `UPDATE release_operations SET updated_at=now()-interval '1 hour' WHERE id=$1`, crash); err != nil {
		t.Fatal(err)
	}
	expired, err := c2.ExpireStale(ctx)
	if err != nil || expired != 1 {
		t.Fatalf("expire: %d %v", expired, err)
	}
	worked, err := c2.RunOnce(ctx)
	if err != nil || !worked {
		t.Fatalf("resume: %v %v", worked, err)
	}
	if state, _ := opState(t, db, crash); state != Ready {
		t.Fatal("crashed operation must finish Ready after reclaim")
	}
	waitGateway(t, "echo2")
	fmt.Println("acceptance scenarios passed: scale, registry outage, controller crash")
}
