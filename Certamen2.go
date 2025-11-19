package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

type XorShift64 struct {
	s uint64
}

func NewXorShift64(seed uint64) *XorShift64 {
	if seed == 0 {
		seed = 1
	}
	return &XorShift64{s: seed}
}

func (x *XorShift64) Uint64() uint64 {
	x.s ^= x.s >> 12
	x.s ^= x.s << 25
	x.s ^= x.s >> 27
	return x.s * 2685821657736338717
}

func (x *XorShift64) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(x.Uint64() % uint64(n))
}

func (x *XorShift64) State() uint64 {
	return x.s
}

func (x *XorShift64) SetState(st uint64) {
	if st == 0 {
		st = 1
	}
	x.s = st
}

type Evento struct {
	MarcaDeTiempo int
	Tipo          string
}

type LogEntry struct {
	WorkerID  int    `json:"worker_id"`
	Action    string `json:"action"`
	Timestamp int    `json:"timestamp_lvt"`
	Details   string `json:"details,omitempty"`
}

type SucursalSaveState struct {
	LVT      int
	RNGState uint64
}

type Sucursal struct {
	ID               int
	State            SucursalSaveState
	Checkpoints      map[int]SucursalSaveState
	HistorialEventos []Evento

	CanalEventos chan Evento
	mu           sync.Mutex
	rng          *XorShift64
	Log          []LogEntry
}

func (t *Sucursal) run(wg *sync.WaitGroup) {
	defer wg.Done()
	for evento := range t.CanalEventos {
		t.mu.Lock()
		t.Log = append(t.Log, LogEntry{
			WorkerID:  t.ID,
			Action:    fmt.Sprintf("Recibida '%s'", evento.Tipo),
			Timestamp: t.State.LVT,
			Details:   fmt.Sprintf("Evento T_E=%d", evento.MarcaDeTiempo),
		})
		if evento.MarcaDeTiempo < t.State.LVT {
			t.eventoatrasado(evento)
		} else {
			t.eventonormal(evento)
		}

		t.mu.Unlock()
	}
}

func (t *Sucursal) eventoatrasado(evento Evento) {
	t.Log = append(t.Log, LogEntry{
		WorkerID:  t.ID,
		Action:    "Rollback Iniciado",
		Timestamp: t.State.LVT,
		Details:   fmt.Sprintf("T_E=%d < LVT=%d", evento.MarcaDeTiempo, t.State.LVT),
	})

	ts, encontrado := t.findCheckpointToRestore(evento.MarcaDeTiempo)
	if encontrado {
		t.State = t.Checkpoints[ts]
		t.rng.SetState(t.State.RNGState)
		t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Rollback: Checkpoint Restaurado", Timestamp: t.State.LVT, Details: fmt.Sprintf("Restaurado a T=%d", t.State.LVT)})
	} else {
		t.State = SucursalSaveState{LVT: 0, RNGState: t.rng.State()}
		t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Rollback: Restaurado a estado inicial", Timestamp: t.State.LVT})
	}

	lvtRestaurado := t.State.LVT
	eventosParaReprocesar := make([]Evento, 0)
	nuevoHistorial := make([]Evento, 0)
	eventosParaReprocesar = append(eventosParaReprocesar, evento)
	nuevoHistorial = append(nuevoHistorial, evento)
	for _, ev := range t.HistorialEventos {
		if ev.MarcaDeTiempo > lvtRestaurado {
			eventosParaReprocesar = append(eventosParaReprocesar, ev)
			nuevoHistorial = append(nuevoHistorial, ev)
		} else if ev.MarcaDeTiempo == lvtRestaurado {
			nuevoHistorial = append(nuevoHistorial, ev)
		}
		if lvtRestaurado == 0 && ev.MarcaDeTiempo < evento.MarcaDeTiempo {
			nuevoHistorial = append(nuevoHistorial, ev)
		}
	}

	sort.Slice(eventosParaReprocesar, func(i, j int) bool {
		return eventosParaReprocesar[i].MarcaDeTiempo < eventosParaReprocesar[j].MarcaDeTiempo
	})
	sort.Slice(nuevoHistorial, func(i, j int) bool { return nuevoHistorial[i].MarcaDeTiempo < nuevoHistorial[j].MarcaDeTiempo })
	t.HistorialEventos = nuevoHistorial
	t.reprocessEvents(eventosParaReprocesar)
	t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Rollback Terminado", Timestamp: t.State.LVT})
}

func (t *Sucursal) findCheckpointToRestore(ts int) (int, bool) {
	timestampParaRestaurar := 0
	encontrado := false
	for timestamp := range t.Checkpoints {
		if timestamp < ts {
			if !encontrado || timestamp > timestampParaRestaurar {
				timestampParaRestaurar = timestamp
				encontrado = true
			}
		}
	}
	return timestampParaRestaurar, encontrado
}

func (t *Sucursal) reprocessEvents(events []Evento) {
	for _, ev := range events {
		if ev.Tipo == "tarea a" {
			t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Rollback: Checkpoint Creado", Timestamp: t.State.LVT})
			t.State.RNGState = t.rng.State()
			t.Checkpoints[t.State.LVT] = t.State
		}
		if ev.MarcaDeTiempo > t.State.LVT {
			t.State.LVT = ev.MarcaDeTiempo
		}
		t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: fmt.Sprintf("Rollback: Reprocesada '%s'", ev.Tipo), Timestamp: t.State.LVT})
		if ev.Tipo == "tarea a" {
			processingTime := t.rng.Intn(5) + 1
			internalTimestamp := t.State.LVT + processingTime
			t.State.LVT = internalTimestamp

			t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Rollback: Reprocesada 'tarea b'", Timestamp: t.State.LVT, Details: fmt.Sprintf("Duración: %d", processingTime)})
		}
	}
}

func (t *Sucursal) eventonormal(evento Evento) {
	if evento.Tipo == "tarea a" {
		t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Checkpoint Creado", Timestamp: t.State.LVT})
		t.State.RNGState = t.rng.State()
		t.Checkpoints[t.State.LVT] = t.State
		t.HistorialEventos = append(t.HistorialEventos, evento)
	}
	t.State.LVT = evento.MarcaDeTiempo
	t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: fmt.Sprintf("Procesada '%s'", evento.Tipo), Timestamp: t.State.LVT})
	if evento.Tipo == "tarea a" {
		processingTime := t.rng.Intn(5) + 1
		internalTimestamp := t.State.LVT + processingTime
		t.State.LVT = internalTimestamp
		t.Log = append(t.Log, LogEntry{WorkerID: t.ID, Action: "Procesada 'tarea b' (Interna)", Timestamp: t.State.LVT, Details: fmt.Sprintf("Duración: %d", processingTime)})
	}
}

func main() {
	seedFlag := flag.Int64("seed", 0, "seed para reproducibilidad (0 => usar time.Now().UnixNano())")
	flag.Parse()
	seed := *seedFlag
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	tiempo_inicial := time.Now()
	const numSucursales = 1
	const numProyectos = 10000
	var wg sync.WaitGroup

	canalesSucursales := make([]chan Evento, numSucursales)
	sucursales := make([]*Sucursal, numSucursales)
	for i := 0; i < numSucursales; i++ {
		canalesSucursales[i] = make(chan Evento, 10)
		rng := NewXorShift64(uint64(seed) + uint64(i) + 1)

		sucursales[i] = &Sucursal{
			ID:               i,
			State:            SucursalSaveState{LVT: 0, RNGState: rng.State()},
			Checkpoints:      make(map[int]SucursalSaveState),
			HistorialEventos: make([]Evento, 0),
			CanalEventos:     canalesSucursales[i],
			rng:              rng,
			Log:              make([]LogEntry, 0),
		}

		wg.Add(1)
		go sucursales[i].run(&wg)
	}

	mainRng := rand.New(rand.NewSource(seed))
	tiempoSedeCentral := 0
	for i := 0; i < numProyectos; i++ {
		tiempoSedeCentral += mainRng.Intn(3) + 1
		sucursalID := mainRng.Intn(numSucursales)
		canalesSucursales[sucursalID] <- Evento{MarcaDeTiempo: tiempoSedeCentral, Tipo: "tarea a"}
	}
	//time.Sleep(500 * time.Millisecond)
	for i := 0; i < numSucursales; i++ {
		close(canalesSucursales[i])
	}

	wg.Wait()
	tiempo_final := time.Since(tiempo_inicial)
	fmt.Printf("\n--- Tiempo Total de la Ejecución: %v ---\n", tiempo_final)
	fmt.Println("--- Simulación Terminada ---")
	fmt.Println("Generando archivo de log (simulacion_log.csv)...")
	if err := datos(sucursales, "simulacion_log.csv"); err != nil {
		fmt.Println("Error al generar CSV:", err)
		return
	}
	fmt.Println("CSV guardado exitosamente: simulacion_log.csv")
}

func datos(sucursales []*Sucursal, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"sucursal_id", "action", "timestamp_lvt", "details"}); err != nil {
		return err
	}
	for _, sucursal := range sucursales {
		for _, entry := range sucursal.Log {
			rec := []string{
				strconv.Itoa(sucursal.ID),
				entry.Action,
				strconv.Itoa(entry.Timestamp),
				entry.Details,
			}
			if err := w.Write(rec); err != nil {
				return err
			}
		}
	}

	return nil
}
