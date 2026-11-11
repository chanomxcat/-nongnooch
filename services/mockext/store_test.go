package main

import (
	"fmt"
	"sync"
	"testing"
)

func TestStoreCapAndOrder(t *testing.T) {
	s := NewStore()
	for i := 0; i < 201; i++ {
		s.Add("sms", Entry{Title: fmt.Sprint(i)})
	}
	l := s.List("sms")
	if len(l) != 200 || l[0].Title != "200" || l[199].Title != "1" {
		t.Fatalf("len=%d first=%s last=%s", len(l), l[0].Title, l[len(l)-1].Title)
	}
	if len(s.List("push")) != 0 {
		t.Fatal("other kind not empty")
	}
}

func TestStoreConcurrent(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.Add("sms", Entry{}); s.List("sms") }()
	}
	wg.Wait()
	if len(s.List("sms")) != 50 {
		t.Fatal("lost entries")
	}
}
