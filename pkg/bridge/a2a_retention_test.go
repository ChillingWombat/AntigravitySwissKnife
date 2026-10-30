package bridge

import (
	"fmt"
	"testing"
	"time"
)

func retentionTask(id, status string, at time.Time) *A2ATask {
	completed := at
	task := &A2ATask{ID: id, Status: status, CreatedAt: at, UpdatedAt: at}
	if isTerminalStatus(status) {
		task.CompletedAt = &completed
	}
	return task
}

func addRetentionTask(manager *A2ATaskManager, task *A2ATask) {
	manager.tasks[task.ID] = task
	manager.order = append([]string{task.ID}, manager.order...)
}

func assertRetentionOrder(t *testing.T, manager *A2ATaskManager) {
	t.Helper()
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if len(manager.tasks) != len(manager.order) {
		t.Fatalf("task/order size mismatch: tasks=%d order=%d", len(manager.tasks), len(manager.order))
	}
	seen := make(map[string]bool, len(manager.order))
	for _, id := range manager.order {
		if manager.tasks[id] == nil {
			t.Fatalf("order contains missing task %q", id)
		}
		if seen[id] {
			t.Fatalf("order contains duplicate task %q", id)
		}
		seen[id] = true
	}
}

func TestA2ATerminalTTLProtectsActiveAndSubscribedTasks(t *testing.T) {
	base := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	manager := NewA2ATaskManager(func() agentBackend { return nil })
	manager.now = func() time.Time { return base.Add(2 * time.Hour) }
	addRetentionTask(manager, retentionTask("expired", TaskStatusCompleted, base))
	addRetentionTask(manager, retentionTask("active", TaskStatusWorking, base))
	addRetentionTask(manager, retentionTask("subscribed", TaskStatusCompleted, base))
	_, unsubscribe := manager.SubscribeTaskEvents("subscribed")

	tasks := manager.ListTasks("", 0)
	if len(tasks) != 2 {
		t.Fatalf("expected active and subscribed tasks, got %d", len(tasks))
	}
	if _, err := manager.GetTask("expired"); err == nil {
		t.Fatal("expired terminal task was not evicted")
	}
	stats := manager.RetentionStats()
	if stats.Total != 2 || stats.Active != 1 || stats.Terminal != 1 || stats.Subscribers != 1 {
		t.Fatalf("unexpected retention stats: %+v", stats)
	}
	if stats.MaxTasks != a2aMaxTasks || stats.TerminalTTLSeconds != int64(a2aTerminalTTL/time.Second) {
		t.Fatalf("retention limits not exposed: %+v", stats)
	}
	assertRetentionOrder(t, manager)

	unsubscribe()
	manager.TaskForContext("subscribed", "")
	if _, err := manager.GetTask("subscribed"); err == nil {
		t.Fatal("unsubscribed expired terminal task was not evicted")
	}
	assertRetentionOrder(t, manager)
}

func TestA2ACapacityRejectsProtectedTasksAndPrunesExpiredTerminal(t *testing.T) {
	base := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	manager := NewA2ATaskManager(func() agentBackend { return newMockBackend() })
	manager.now = func() time.Time { return base.Add(2 * time.Hour) }
	addRetentionTask(manager, retentionTask("expired-oldest", TaskStatusFailed, base))
	for i := 0; i < a2aMaxTasks-1; i++ {
		addRetentionTask(manager, retentionTask(fmt.Sprintf("active-%03d", i), TaskStatusSubmitted, base))
	}
	if stats := manager.RetentionStats(); stats.Total != a2aMaxTasks-1 {
		t.Fatalf("expired terminal task should be swept before create, got %+v", stats)
	}
	created, err := manager.CreateTask(SkillSoftwareEngineering, "prove bounded retention", "retention")
	if err != nil {
		t.Fatalf("expired terminal task should have made room: %v", err)
	}
	if _, err := manager.GetTask("expired-oldest"); err == nil {
		t.Fatal("oldest expired terminal task survived capacity sweep")
	}
	if stats := manager.RetentionStats(); stats.Total != a2aMaxTasks {
		t.Fatalf("expected capacity after create, got %+v", stats)
	}
	manager.mu.RLock()
	_, oldPresent := manager.tasks["expired-oldest"]
	_, newPresent := manager.tasks[created.ID]
	manager.mu.RUnlock()
	if oldPresent || !newPresent {
		t.Fatalf("capacity replacement inconsistent: old=%v new=%v", oldPresent, newPresent)
	}
	assertRetentionOrder(t, manager)

	blocked := NewA2ATaskManager(func() agentBackend { return nil })
	blocked.now = func() time.Time { return base.Add(2 * time.Hour) }
	for i := 0; i < a2aMaxTasks-1; i++ {
		addRetentionTask(blocked, retentionTask(fmt.Sprintf("blocked-%03d", i), TaskStatusWorking, base))
	}
	addRetentionTask(blocked, retentionTask("protected-terminal", TaskStatusCompleted, base))
	_, unsubscribe := blocked.SubscribeTaskEvents("protected-terminal")
	if _, err := blocked.CreateTask(SkillSoftwareEngineering, "must reject", "retention"); err == nil {
		t.Fatal("capacity should reject when every terminal task has an active stream")
	}
	if stats := blocked.RetentionStats(); stats.Total != a2aMaxTasks || stats.Subscribers != 1 {
		t.Fatalf("rejected create mutated retained state: %+v", stats)
	}
	unsubscribe()
	assertRetentionOrder(t, blocked)
}
