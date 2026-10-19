package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// TaskLoop drains the transactional outbox. A restart or sink failure retries
// the same event ID; the durable inbox remains accessible even without push.
type TaskLoop struct {
	Store storage.Driver
	Hub   *Hub
	Log   *slog.Logger
}

func (l *TaskLoop) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			if err := l.drain(ctx); err != nil && ctx.Err() == nil && l.Log != nil {
				l.Log.Warn("task notifications", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (l *TaskLoop) drain(ctx context.Context) error {
	rows, err := l.Store.PendingTaskNotifications(ctx)
	if err != nil {
		return err
	}
	for _, n := range rows {
		if err = ctx.Err(); err != nil {
			return err
		}
		if n.ReadAt == "" {
			created, _ := time.Parse(time.RFC3339Nano, n.CreatedAt)
			event := Event{ID: n.ID, Kind: KindTask, Title: "Изменение задачи", Body: taskNoticeText(n.Action), Href: "tasks", CreatedAt: created}
			if err = l.Hub.Publish(n.UserID, event); err != nil {
				return err
			}
		}
		if err = l.Store.DispatchTaskNotification(ctx, n.ID); err != nil {
			return err
		}
	}
	return nil
}
func taskNoticeText(action string) string {
	switch action {
	case "assign":
		return "Вам назначена задача. Примите или отклоните её в разделе «Задачи»."
	case "accept":
		return "Исполнитель принял задачу."
	case "decline":
		return "Исполнитель отклонил задачу."
	case "complete":
		return "Задача завершена."
	case "reopen":
		return "Задача снова открыта."
	case "revoke":
		return "Назначение задачи отозвано."
	case "delete":
		return "Автор удалил задачу."
	default:
		return "Состояние или описание задачи изменено."
	}
}
