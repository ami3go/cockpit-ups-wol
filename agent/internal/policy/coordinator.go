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
		// Keep in-memory state aligned with the last state we know was durable.
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
