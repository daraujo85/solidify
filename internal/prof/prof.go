// Memory profiling helpers (SAI-097).
//
// Identifica cópias de diff/artifacts e mede alocações
// em hot paths. API mínima: mede bytes alocados e número
// de GC runs durante uma operação.
package prof

import (
	"runtime"
	"time"
)

// Snapshot estado de memória.
type Snapshot struct {
	HeapAlloc   uint64    // bytes alocados no heap
	HeapObjects uint64    // objetos vivos
	NumGC       uint32    // GC runs até agora
	Sys         uint64    // total bytes do runtime
	Timestamp   time.Time // quando o snapshot foi tirado
}

// TakeSnapshot lê estado atual via runtime.MemStats.
func TakeSnapshot() Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return Snapshot{
		HeapAlloc:   m.HeapAlloc,
		HeapObjects: m.HeapObjects,
		NumGC:       m.NumGC,
		Sys:         m.Sys,
		Timestamp:   time.Now().UTC(),
	}
}

// Diff entre dois snapshots (after - before).
//
// Retorna:
// - AllocDelta: bytes alocados no período.
// - ObjectsDelta: objetos criados no período.
// - GCDelta: GC runs no período.
// - Elapsed: tempo entre snapshots.
func Diff(before, after Snapshot) DiffResult {
	return DiffResult{
		AllocDelta:   int64(after.HeapAlloc) - int64(before.HeapAlloc),
		ObjectsDelta: int64(after.HeapObjects) - int64(before.HeapObjects),
		GCDelta:      after.NumGC - before.NumGC,
		Elapsed:      after.Timestamp.Sub(before.Timestamp),
	}
}

// DiffResult saída de Diff.
type DiffResult struct {
	AllocDelta   int64
	ObjectsDelta int64
	GCDelta      uint32
	Elapsed      time.Duration
}

// Measure roda fn e mede alocações.
//
// Uso típico:
//
//	before := prof.TakeSnapshot()
//	fn()
//	after := prof.TakeSnapshot()
//	d := prof.Diff(before, after)
//	fmt.Printf("alloc=%d obj=%d gc=%d\n", d.AllocDelta, d.ObjectsDelta, d.GCDelta)
func Measure(fn func()) DiffResult {
	runtime.GC()
	before := TakeSnapshot()
	fn()
	after := TakeSnapshot()
	return Diff(before, after)
}

// CopyDetector identifica se um byte slice foi copiado.
//
// Útil p/ validar que streaming evitou cópia. Estratégia:
// compara primeiros 8 bytes — se diferirem, houve
// realocação/cópia. Limitação: se a cópia preservar
// header, não detecta.
type CopyDetector struct {
	id      uint64
	witness []byte
}

// NewCopyDetector cria detector.
func NewCopyDetector(data []byte) *CopyDetector {
	return &CopyDetector{id: nextID(), witness: data}
}

// WasCopied retorna true se slice parece ter sido
// modificado (cópia ou alteração). Heurística: compara
// primeiros 8 bytes.
func (cd *CopyDetector) WasCopied(current []byte) bool {
	if len(current) < 8 || len(cd.witness) < 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		if current[i] != cd.witness[i] {
			return true
		}
	}
	return false
}

var counter uint64

func nextID() uint64 {
	counter++
	return counter
}

// BufferPool pool de []byte p/ reduzir cópias.
//
// Uso:
//
//	pool := prof.NewBufferPool(64 * 1024)
//	buf := pool.Get()
//	defer pool.Put(buf)
type BufferPool struct {
	size int
	pool chan []byte
}

// NewBufferPool cria pool de buffers de N bytes.
func NewBufferPool(bufSize int) *BufferPool {
	return &BufferPool{
		size: bufSize,
		pool: make(chan []byte, 32),
	}
}

// Get retorna buffer do pool (ou novo se vazio).
func (p *BufferPool) Get() []byte {
	select {
	case b := <-p.pool:
		return b
	default:
		return make([]byte, 0, p.size)
	}
}

// Put devolve buffer ao pool.
func (p *BufferPool) Put(b []byte) {
	if cap(b) < p.size {
		return // descarta buffers menores
	}
	select {
	case p.pool <- b[:0]:
	default:
		// pool cheio — descarta.
	}
}

// Size retorna tamanho configurado.
func (p *BufferPool) Size() int { return p.size }

// PoolLen retorna buffers atualmente no pool.
func (p *BufferPool) PoolLen() int { return len(p.pool) }
