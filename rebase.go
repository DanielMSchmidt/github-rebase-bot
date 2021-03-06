package main

import (
	"log"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

type WorkerCache interface {
	Worker(string) (repo.Enqueuer, error)
	Update() (string, error)
}

func processRebase(cache WorkerCache, in <-chan *github.PullRequest) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)

	input := make(chan *github.PullRequest)
	go func() {
		for pr := range in {
			input <- pr
		}
		close(input)
	}()

	go func() {
		for pr := range input {
			w, err := cache.Worker(pr.Head.GetRef())
			if err != nil {
				continue
			}

			c := make(chan repo.Signal, 1)

			rev, err := cache.Update()
			if err != nil {
				log.Printf("failed to update master: %v", err)
				continue
			}

			w.Enqueue(c)
			go func(pr *github.PullRequest, rev string) {
				sig := <-c

				rev2, _ := cache.Update()
				if rev != rev2 {
					// master changed while we were processing this PR. re-process to handle cont. rebasing
					input <- pr
					return
				}

				if sig.UpToDate && sig.Error == nil {
					ret <- pr
				}
			}(pr, rev)
		}

		close(ret)
	}()

	return ret
}
