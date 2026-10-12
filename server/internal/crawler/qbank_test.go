package crawler

import (
	"testing"

	"gongkao/internal/service"
)

func TestParseQuestionPayloadNative(t *testing.T) {
	raw := []byte(`[
	  {"category":"行测-数量关系","stem":"1+1=?","answer_key":"A","analysis":"2",
	   "options":[{"key":"A","content":"2"},{"key":"B","content":"3"},{"key":"C","content":"4"},{"key":"D","content":"5"}]}
	]`)
	items, err := parseQuestionPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].AnswerKey != "A" {
		t.Fatalf("unexpected: %+v", items)
	}
}

func TestParseQuestionPayloadAlt(t *testing.T) {
	raw := []byte(`[
	  {"title":"题干","answer":"B","analysis":"解析","module":"言语理解",
	   "options":[{"text":"甲"},{"text":"乙"},{"text":"丙"},{"text":"丁"}]}
	]`)
	items, err := parseQuestionPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	it := normalizeImportQuestion(items[0])
	if it.Category != "行测-言语理解" || it.AnswerKey != "B" || len(it.Options) != 4 {
		t.Fatalf("unexpected: %+v", it)
	}
}

func TestMapModuleCategory(t *testing.T) {
	if mapModuleCategory("法律基础") != "公基-法律基础" {
		t.Fatal(mapModuleCategory("法律基础"))
	}
}

func TestNormalizeImportQuestion(t *testing.T) {
	it := normalizeImportQuestion(service.ImportQuestion{
		Category:  "时政",
		Stem:      " 题 ",
		AnswerKey: "ab",
		Options:   []map[string]string{{"content": "x"}, {"key": "B", "content": "y"}},
	})
	if it.AnswerKey != "A" || it.Stem != "题" || it.Category != "时政常识" {
		t.Fatalf("%+v", it)
	}
}
