package service

import "fmt"

const (
	HealthStatusBleached    = "bleached"
	HealthStatusNotBleached = "not_bleached"
)

// HealthStatus records bleaching only; not_bleached is not a general health
// assessment. NULL means no point-level answer, never an implicit negative.
// The old boolean remains an explicitly synchronized compatibility field.
func (p *ReefCheckSubstratePointInput) normalizeHealthStatus() error {
	if p.HealthStatus != nil {
		if *p.HealthStatus != HealthStatusBleached && *p.HealthStatus != HealthStatusNotBleached {
			return fmt.Errorf("%w: health_status 只接受 bleached、not_bleached 或 null", ErrValidation)
		}
		bleached := *p.HealthStatus == HealthStatusBleached
		if p.IsBleached != nil && *p.IsBleached != bleached {
			return fmt.Errorf("%w: health_status 與 is_bleached 不一致", ErrValidation)
		}
		p.IsBleached = &bleached
	} else if p.IsBleached != nil {
		status := HealthStatusNotBleached
		if *p.IsBleached {
			status = HealthStatusBleached
		}
		p.HealthStatus = &status
	}
	return nil
}
