package policy

import (
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

// StateWriter durably persists a complete power-transaction state generation.
type StateWriter interface {
	Write(state.State) (state.State, error)
}

// Coordinator couples the deterministic policy engine to durable state writes.
// A changed decision is never returned to an external-action caller until the
// resulting state generation has been written successfully.
type Coordinator struct {
	engine *Engine
	store  StateWriter
}

func NewCoordinator(engine *Engine, store StateWriter) *Coordinator {
	return &Coordinator{engine: engine, store: store}
}

func (c *Coordinator) State() state.State { return c.engine.State() }

// ObserveUPS refreshes the in-memory UPS observation carried into the next
// durable state generation. It deliberately does not force a write on every
// poll; every subsequent durable write therefore records the observation
// without adding a five-second flash/disk write loop.
func (c *Coordinator) ObserveUPS(observation state.UPSObservation) {
	c.engine.st.LastUPS = &observation
}

// Write makes Coordinator itself a state writer. This is used by host/recovery
// orchestration so every durable per-host update also refreshes the policy
// engine's in-memory state; the two views can therefore never drift apart.
func (c *Coordinator) Write(next state.State) (state.State, error) {
	persisted, err := c.store.Write(next)
	if err != nil {
		return state.State{}, err
	}
	c.engine.st = persisted
	return persisted, nil
}

func (c *Coordinator) Step(now time.Time, in Inputs) (Decision, error) {
	before := c.engine.State()
	decision, err := c.engine.Step(now, in)
	if err != nil {
		return Decision{}, err
	}
	if !decision.Changed {
		return decision, nil
	}
	persisted, err := c.store.Write(c.engine.State())
	if err != nil {
		c.engine.st = before
		return Decision{}, err
	}
	c.engine.st = persisted
	return decision, nil
}

func (c *Coordinator) MarkShutdownPhaseComplete() (Decision, error) {
	before := c.engine.State()
	decision := c.engine.MarkShutdownPhaseComplete()
	if !decision.Changed {
		return decision, nil
	}
	persisted, err := c.store.Write(c.engine.State())
	if err != nil {
		c.engine.st = before
		return Decision{}, err
	}
	c.engine.st = persisted
	return decision, nil
}

func (c *Coordinator) MarkRecoveryComplete() (Decision, error) {
	before := c.engine.State()
	decision := c.engine.MarkRecoveryComplete()
	if !decision.Changed {
		return decision, nil
	}
	persisted, err := c.store.Write(c.engine.State())
	if err != nil {
		c.engine.st = before
		return Decision{}, err
	}
	c.engine.st = persisted
	return decision, nil
}
