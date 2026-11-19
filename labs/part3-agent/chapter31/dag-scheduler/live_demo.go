package main

import "context"

func liveDemo(ctx context.Context) (ResourceReport, error) {
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	input := make(chan ReadyJob)
	started := make(chan struct{})
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		defer close(input)
		for n, id := range []string{"a1", "a2", "a3", "a4"} {
			job := ReadyJob{ID: id, Tenant: "a", Needs: []string{"db"}, Run: func(ctx context.Context) (string, error) { return "read", nil }}
			if n == 0 {
				job.Run = func(ctx context.Context) (string, error) { close(started); <-ctx.Done(); return "late", nil }
			}
			select {
			case input <- job:
			case <-life.Done():
				return
			}
			if n == 0 {
				select {
				case <-started:
				case <-life.Done():
					return
				}
			}
		}
	}()
	report, err := RunReadyStream(life, input, map[string]int{"db": 1}, 1, 1, 4)
	cancel()
	<-producerDone
	return report, err
}
