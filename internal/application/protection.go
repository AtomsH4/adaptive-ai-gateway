package application

import (
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"time"
)

func (c *Coordinator) updateProtection(signal domain.ResourceSignal, sample, now time.Time) {
	if signal.Kind == domain.SignalCritical {
		if c.criticalAt.IsZero() {
			c.criticalAt = now
		}
		c.protection = "reclaiming"
		c.reason = "memory_pressure"
		c.normalSamples = 0
		c.normalSince = time.Time{}
		return
	}
	if c.reclaim {
		if c.instance.ID != "" {
			c.protection = "reclaiming"
			return
		}
		c.reclaim = false
		c.criticalAt = time.Time{}
	}
	if !c.criticalAt.IsZero() {
		if signal.Kind == domain.SignalObservationGuard {
			return
		}
		c.criticalAt = time.Time{}
		c.wasCritical = true
	}
	if signal.Kind != domain.SignalNormal {
		c.normalSamples = 0
		c.normalSince = time.Time{}
		c.lastNormalSample = time.Time{}
		c.reason = string(signal.Reason)
		c.protection = "constrained"
		if signal.Kind == domain.SignalObservationGuard {
			c.protection = "observation_guard"
		}
		return
	}
	if c.normalSince.IsZero() {
		c.normalSince = now
	}
	if sample.After(c.lastNormalSample) {
		c.normalSamples++
		c.lastNormalSample = sample
	}
	if c.normalSamples < 2 || (c.wasCritical && now.Sub(c.normalSince) < c.opts.RecoveryWindow) {
		c.protection = "recovering"
		return
	}
	c.protection = "normal"
	c.reason = ""
	c.wasCritical = false
}
