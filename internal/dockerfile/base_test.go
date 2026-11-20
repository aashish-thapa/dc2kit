package dockerfile

import (
	"strings"
	"testing"
)

func TestPrepareNamesTheLastStage(t *testing.T) {
	b, err := Prepare([]byte("ARG VARIANT=22\nFROM node:${VARIANT} AS deps\nRUN npm ci\nFROM mcr.microsoft.com/devcontainers/base:ubuntu\nCOPY --from=deps /x /x\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if b.Stage != DefaultStage || !strings.Contains(b.Body, "FROM mcr.microsoft.com/devcontainers/base:ubuntu AS "+DefaultStage+"\n") {
		t.Fatalf("stage %q, body:\n%s", b.Stage, b.Body)
	}
	if !strings.HasPrefix(b.Body, "ARG VARIANT=22\n") {
		t.Fatal("global ARGs before the first FROM must stay first")
	}
}

func TestPrepareKeepsAnExistingAliasAndContinuations(t *testing.T) {
	b, err := Prepare([]byte("FROM --platform=$BUILDPLATFORM \\\n    ubuntu:24.04 \\\n    AS dev\nRUN true\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if b.Stage != "dev" || strings.Contains(b.Body, DefaultStage) {
		t.Fatalf("stage %q, body:\n%s", b.Stage, b.Body)
	}
}

func TestPrepareTarget(t *testing.T) {
	src := []byte("FROM ubuntu AS dev\nFROM dev AS ci\n")
	b, err := Prepare(src, "DEV")
	if err != nil || b.Stage != "dev" {
		t.Fatalf("stage %q, err %v", b.Stage, err)
	}
	if _, err := Prepare(src, "prod"); err == nil {
		t.Fatal("an unknown target must fail")
	}
}

func TestPrepareDirectives(t *testing.T) {
	b, err := Prepare([]byte("# syntax=docker/dockerfile:1.7\n# check=error=true\nFROM ubuntu\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Header) != 2 || !b.HasSyntax() || strings.Contains(b.Body, "syntax=") {
		t.Fatalf("header %q body %q", b.Header, b.Body)
	}
	if _, err := Prepare([]byte("# escape=`\nFROM ubuntu\n"), ""); err == nil {
		t.Fatal("a backtick escape would break appended instructions")
	}
	if _, err := Prepare([]byte("# just a comment\nRUN true\n"), ""); err == nil {
		t.Fatal("no FROM must fail")
	}
}
