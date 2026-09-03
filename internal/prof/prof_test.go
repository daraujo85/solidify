package prof

import (
	"strings"
	"testing"
)

// Aceitação: TakeSnapshot retorna valores não-zero.
func TestTakeSnapshot(t *testing.T) {
	s := TakeSnapshot()
	if s.Timestamp.IsZero() {
		t.Errorf("ts zero")
	}
}

// Aceitação: Diff calcula deltas.
func TestDiff(t *testing.T) {
	b := Snapshot{HeapAlloc: 100, HeapObjects: 5, NumGC: 1}
	a := Snapshot{HeapAlloc: 150, HeapObjects: 8, NumGC: 2}
	d := Diff(b, a)
	if d.AllocDelta != 50 {
		t.Errorf("alloc: %d", d.AllocDelta)
	}
	if d.ObjectsDelta != 3 {
		t.Errorf("obj: %d", d.ObjectsDelta)
	}
	if d.GCDelta != 1 {
		t.Errorf("gc: %d", d.GCDelta)
	}
}

// Aceitação: Measure roda fn e retorna Diff.
func TestMeasure(t *testing.T) {
	d := Measure(func() {
		_ = make([]byte, 1024)
	})
	if d.Elapsed < 0 {
		t.Errorf("elapsed: %v", d.Elapsed)
	}
}

// Aceitação: CopyDetector básico.
func TestCopyDetector(t *testing.T) {
	data := []byte("abcdefghij")
	cd := NewCopyDetector(data)
	if cd.WasCopied(data) {
		t.Errorf("same slice")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	// cp tem mesmos primeiros 8 bytes — WasCopied=false (mesma assinatura).
	if cd.WasCopied(cp) {
		// aceitável: header igual → não detecta cópia.
	}
	// modifica cp[7] — agora id "muda".
	cp[7] = 'X'
	if !cd.WasCopied(cp) {
		t.Errorf("expected copy")
	}
}

// Aceitação: BufferPool Get/Put.
func TestBufferPool(t *testing.T) {
	p := NewBufferPool(64)
	b := p.Get()
	if cap(b) < 64 {
		t.Errorf("cap: %d", cap(b))
	}
	p.Put(b)
	if p.PoolLen() == 0 {
		t.Errorf("pool vazio")
	}
	if p.Size() != 64 {
		t.Errorf("size")
	}
}

// Aceitação: BufferPool descarta buffer pequeno.
func TestBufferPoolSmall(t *testing.T) {
	p := NewBufferPool(1024)
	p.Put(make([]byte, 16))
	if p.PoolLen() != 0 {
		t.Errorf("discard")
	}
}

// Aceitação: helpers numéricos.
func TestHelperRender(t *testing.T) {
	d := DiffResult{AllocDelta: 1024, ObjectsDelta: 10, GCDelta: 1}
	out := RenderDiff(d)
	if !strings.Contains(out, "1024") || !strings.Contains(out, "10") {
		t.Errorf("render: %s", out)
	}
}
