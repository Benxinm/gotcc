package manager

import "time"

type Options struct {
	//limitation of transaction running time
	Timeout time.Duration
	//gap between each tick
	MonitorTick time.Duration
}

type Option func(*Options)

func WithTimeout(timeout time.Duration) Option {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return func(options *Options) {
		options.Timeout = timeout
	}
}

func WithMonitorTick(tick time.Duration) Option {
	if tick <= 0 {
		tick = 10 * time.Second
	}
	return func(options *Options) {
		options.MonitorTick = tick
	}
}

// Normalize parameters
func repair(o *Options) {
	if o.MonitorTick <= 0 {
		o.MonitorTick = 10 * time.Second
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
}
