package apicontract

import (
	"github.com/pangu-studio/mozi-builder/mozi"
	"strings"
	"testing"
)

func TestGenerateBruno(t *testing.T) {
	m := &mozi.ModelIR{APIIntent: mozi.APIIntentConfig{TestContracts: []mozi.TestContractIR{{Name: "create_deck", OperationID: "createDeck", Expect: mozi.TestExpectationIR{Status: 201}}}}}
	a, err := GenerateBruno(m, []Endpoint{{OperationID: "createDeck", Method: "POST", Path: "/decks"}})
	if err != nil || len(a) != 1 || !strings.Contains(a[0].Content, "status is 201") {
		t.Fatalf("a=%#v err=%v", a, err)
	}
}
