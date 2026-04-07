package handlers

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestPrepareRerankRequestAcceptsStringDocuments(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody([]byte(`{
		"model":"vllm/qwen3-8b-reranker",
		"query":"什么是深度学习？",
		"documents":[
			"深度学习是机器学习的一个子集，基于人工神经网络。",
			"今天中午吃红烧肉。",
			"苹果是一种水果，富含维生素。"
		],
		"top_n":3
	}`))

	req, bifrostReq, err := prepareRerankRequest(ctx)
	if err != nil {
		t.Fatalf("prepareRerankRequest() error = %v", err)
	}
	if req == nil || bifrostReq == nil {
		t.Fatal("expected rerank requests to be returned")
	}
	if len(req.Documents) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(req.Documents))
	}
	if req.Documents[0].Text != "深度学习是机器学习的一个子集，基于人工神经网络。" {
		t.Fatalf("unexpected first document text: %q", req.Documents[0].Text)
	}
	if bifrostReq.Provider != "vllm" {
		t.Fatalf("expected provider vllm, got %q", bifrostReq.Provider)
	}
	if bifrostReq.Model != "qwen3-8b-reranker" {
		t.Fatalf("expected model qwen3-8b-reranker, got %q", bifrostReq.Model)
	}
	if bifrostReq.Params == nil || bifrostReq.Params.TopN == nil || *bifrostReq.Params.TopN != 3 {
		t.Fatal("expected top_n=3 to be parsed")
	}
}

func TestPrepareRerankRequestAcceptsDocumentObjects(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody([]byte(`{
		"model":"vllm/BAAI-bge-reranker-v2-m3",
		"query":"test query",
		"documents":[
			{"text":"doc-1","id":"a"},
			{"text":"doc-2","meta":{"lang":"zh"}}
		]
	}`))

	req, bifrostReq, err := prepareRerankRequest(ctx)
	if err != nil {
		t.Fatalf("prepareRerankRequest() error = %v", err)
	}
	if req == nil || bifrostReq == nil {
		t.Fatal("expected rerank requests to be returned")
	}
	if len(req.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(req.Documents))
	}
	if req.Documents[0].ID == nil || *req.Documents[0].ID != "a" {
		t.Fatal("expected first document id to be preserved")
	}
	if req.Documents[1].Meta["lang"] != "zh" {
		t.Fatalf("expected second document meta to be preserved, got %#v", req.Documents[1].Meta)
	}
}
