// Package rpcexample is the phase-3 runnable RPC sample: the .proto contract
// is rendered from a fixture ServiceIR, parsed by protocompile (pure Go, no
// protoc), and executed over a real in-process gRPC round trip.
package rpcexample

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/generator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/test/bufconn"
)

// fixtureService mirrors the docs/v2/service-ir.md example, with RPC enabled.
func fixtureService() *mozi.ServiceIR {
	return &mozi.ServiceIR{
		SchemaVersion: 1,
		Module:        "content",
		Name:          "ContentService",
		Label:         "内容服务",
		Messages: []mozi.MessageIR{
			{
				Name: "DeckSummary",
				Fields: []mozi.MessageFieldIR{
					{Name: "id", Type: "string", Number: 1},
					{Name: "title", Type: "string", Number: 2},
					{Name: "due_at", Type: "time", Number: 3},
				},
				ReservedNumbers: []int32{4},
				ReservedNames:   []string{"review_count"},
			},
			{
				Name: "GetDeckRequest",
				Fields: []mozi.MessageFieldIR{
					{Name: "id", Type: "string", Number: 1},
				},
			},
		},
		RPC: []mozi.RPCMethodIR{
			{Name: "GetDeck", Request: "GetDeckRequest", Response: "DeckSummary"},
		},
	}
}

// renderProto renders the fixture ServiceIR to proto3 source.
func renderProto(t *testing.T) string {
	t.Helper()
	sub, err := fs.Sub(mozi.EmbeddedTemplates, "templates")
	if err != nil {
		t.Fatal(err)
	}
	out, err := generator.NewEngine(sub).ExecuteService("service/proto.tmpl", fixtureService())
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestProtoContractParses validates the rendered contract with protocompile:
// field numbers come from the IR verbatim and deleted fields stay reserved.
func TestProtoContractParses(t *testing.T) {
	src := renderProto(t)
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: func(path string) (io.ReadCloser, error) {
				if path != "content.proto" {
					return nil, fmt.Errorf("unexpected import %s", path)
				}
				return io.NopCloser(strings.NewReader(src)), nil
			},
		}),
	}
	files, err := compiler.Compile(context.Background(), "content.proto")
	if err != nil {
		t.Fatalf("rendered proto must compile: %v\n%s", err, src)
	}
	fd := files[0]

	msg := fd.Messages().ByName("DeckSummary")
	if msg == nil {
		t.Fatal("DeckSummary missing")
	}
	if msg.Fields().ByName("title").Number() != 2 {
		t.Fatal("title must keep IR number 2")
	}
	foundReservedNumber := false
	for i := 0; i < msg.ReservedRanges().Len(); i++ {
		r := msg.ReservedRanges().Get(i) // [start, end), end exclusive
		if r[0] == 4 && r[1] == 5 {
			foundReservedNumber = true
		}
	}
	if !foundReservedNumber {
		t.Fatal("number 4 must stay reserved")
	}
	foundReservedName := false
	for i := 0; i < msg.ReservedNames().Len(); i++ {
		if msg.ReservedNames().Get(i) == "review_count" {
			foundReservedName = true
		}
	}
	if !foundReservedName {
		t.Fatal("review_count must stay reserved")
	}

	svc := fd.Services().ByName("ContentService")
	if svc == nil || svc.Methods().ByName("GetDeck") == nil {
		t.Fatal("ContentService.GetDeck missing")
	}
}

// --- gRPC round trip over a JSON codec, driven by the rendered contract ---

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) String() string {
	return "json"
}
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

var _ encoding.Codec = jsonCodec{}

type deckSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	DueAt int64  `json:"due_at"`
}
type getDeckRequest struct {
	ID string `json:"id"`
}

type contentService interface {
	GetDeck(context.Context, *getDeckRequest) (*deckSummary, error)
}

type contentServiceImpl struct{}

func (contentServiceImpl) GetDeck(_ context.Context, req *getDeckRequest) (*deckSummary, error) {
	return &deckSummary{ID: req.ID, Title: "示例牌组", DueAt: 1700000000000}, nil
}

func getDeckHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(getDeckRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(contentService).GetDeck(ctx, req)
}

// TestRPCExampleServes runs a real gRPC round trip: the service name and
// method come from the rendered contract, executed over bufconn.
func TestRPCExampleServes(t *testing.T) {
	src := renderProto(t)
	if !strings.Contains(src, "rpc GetDeck(GetDeckRequest) returns (DeckSummary);") {
		t.Fatalf("contract drifted from handler:\n%s", src)
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.CustomCodec(jsonCodec{}))
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "content.ContentService",
		HandlerType: (*contentService)(nil),
		Methods:     []grpc.MethodDesc{{MethodName: "GetDeck", Handler: getDeckHandler}},
	}, contentServiceImpl{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp := new(deckSummary)
	err = conn.Invoke(context.Background(), "/content.ContentService/GetDeck",
		&getDeckRequest{ID: "deck-1"}, resp, grpc.ForceCodec(jsonCodec{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "deck-1" || resp.Title != "示例牌组" || resp.DueAt != 1700000000000 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
