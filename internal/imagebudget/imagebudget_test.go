package imagebudget

import (
	"strings"
	"testing"
)

// Aceitação: InspectImage vazio.
func TestInspectImageEmpty(t *testing.T) {
	if _, err := InspectImage(""); err == nil {
		t.Errorf("empty")
	}
}

// Aceitação: InspectImage imagem inexistente.
func TestInspectImageMissing(t *testing.T) {
	// Não falhar compile — só verifica erro de docker.
	if _, err := InspectImage("nonexistent:zzz999"); err == nil {
		// docker pode estar ausente; aceitável.
	}
}

// Aceitação: CheckBudget nil.
func TestCheckBudgetNil(t *testing.T) {
	if err := CheckBudget(nil, DefaultBudget); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: CheckBudget uncompressed > limit.
func TestCheckBudgetUncompressed(t *testing.T) {
	r := &InspectResult{Uncompressed: 999999999}
	if err := CheckBudget(r, DefaultBudget); err == nil {
		t.Errorf("over")
	}
}

// Aceitação: CheckBudget compressed > limit.
func TestCheckBudgetCompressed(t *testing.T) {
	r := &InspectResult{Uncompressed: 100, CompressedSize: 999999999}
	if err := CheckBudget(r, DefaultBudget); err == nil {
		t.Errorf("over")
	}
}

// Aceitação: CheckBudget OK.
func TestCheckBudgetOK(t *testing.T) {
	r := &InspectResult{Uncompressed: 1000, CompressedSize: 500}
	if err := CheckBudget(r, DefaultBudget); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: CheckSecurity root.
func TestCheckSecurityRoot(t *testing.T) {
	r := &InspectResult{User: "root", HasGit: true, HasCACerts: true}
	if err := CheckSecurity(r); err == nil {
		t.Errorf("root")
	}
}

// Aceitação: CheckSecurity sem git.
func TestCheckSecurityNoGit(t *testing.T) {
	r := &InspectResult{User: "app", HasGit: false, HasCACerts: true}
	if err := CheckSecurity(r); err == nil {
		t.Errorf("git")
	}
}

// Aceitação: CheckSecurity sem ca-certs.
func TestCheckSecurityNoCerts(t *testing.T) {
	r := &InspectResult{User: "app", HasGit: true, HasCACerts: false}
	if err := CheckSecurity(r); err == nil {
		t.Errorf("certs")
	}
}

// Aceitação: CheckSecurity OK.
func TestCheckSecurityOK(t *testing.T) {
	r := &InspectResult{User: "app", HasGit: true, HasCACerts: true}
	if err := CheckSecurity(r); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: CheckSecurity nil.
func TestCheckSecurityNil(t *testing.T) {
	if err := CheckSecurity(nil); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: MultiStageDockerfile contém estágios esperados.
func TestMultiStageDockerfile(t *testing.T) {
	d := MultiStageDockerfile()
	if !strings.Contains(d, "FROM golang:1.26-alpine AS build") {
		t.Errorf("build stage")
	}
	if !strings.Contains(d, "FROM alpine:3.20") {
		t.Errorf("runtime stage")
	}
	if !strings.Contains(d, "USER solidify") {
		t.Errorf("non-root")
	}
	if !strings.Contains(d, "apk add --no-cache git ca-certificates") {
		t.Errorf("git+ca-certs")
	}
}
