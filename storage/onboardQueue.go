package storage

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"
)

// OnboardQueue 登岛队列
type OnboardQueue struct {
	ID            string  `firestore:"-"`
	Name          string  `firestore:"Name"`
	Password      string  `firestore:"Password"`
	MaxGuestCount int     `firestore:"MaxGuestCount"`
	Queue         []int64 `firestore:"queue"` //private chat id
	Dismissed     bool    `firestore:"Dismissed"`
}

// GetOnboardQueue return a exists OnboardQueue
func GetOnboardQueue(ctx context.Context, client *firestore.Client, queueID string) (queue *OnboardQueue, err error) {
	snap, err := client.Doc("onboardQueues/" + queueID).Get(ctx)
	if err != nil {
		return
	}
	queue = &OnboardQueue{}
	if err = snap.DataTo(queue); err != nil {
		return
	}
	queue.ID = queueID
	return
}

// Update OnboardQueue into firestore
func (q *OnboardQueue) Update(ctx context.Context, client *firestore.Client) (err error) {
	if q == nil || len(q.ID) == 0 {
		return errors.New("queue not exists")
	}
	_, err = client.Doc("onboardQueues/"+q.ID).Set(ctx, q)
	return
}

// Delete this queue
func (q *OnboardQueue) Delete(ctx context.Context, client *firestore.Client) (err error) {
	if q == nil || len(q.ID) == 0 {
		return errors.New("queue not exists")
	}
	_, err = client.Doc("onboardQueues/" + q.ID).Delete(ctx)
	return
}

// Len return length of OnboardQueue
func (q *OnboardQueue) Len() int {
	if q == nil || len(q.ID) == 0 {
		return 0
	}
	return len(q.Queue)
}

// Append chatID into OnboardQueue
func (q *OnboardQueue) Append(ctx context.Context, client *firestore.Client, chatID int64) (err error) {
	if q == nil || len(q.ID) == 0 {
		return
	}
	if q.Dismissed {
		return errors.New("queue has been dismissed")
	}
	for _, cid := range q.Queue {
		if cid == chatID {
			return errors.New("already in this queue")
		}
	}
	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayUnion(chatID)},
	})
	if err != nil {
		return
	}
	return
}

// Remove chatID into OnboardQueue
func (q *OnboardQueue) Remove(ctx context.Context, client *firestore.Client, chatID int64) (err error) {
	if q == nil || len(q.ID) == 0 {
		return
	}
	if q.Dismissed {
		return errors.New("queue has been dismissed")
	}
	for _, cid := range q.Queue {
		if cid == chatID {
			return errors.New("already in this queue")
		}
	}
	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayRemove(chatID)},
	})
	if err != nil {
		return
	}
	return
}

// Next return next chatID
func (q *OnboardQueue) Next(ctx context.Context, client *firestore.Client) (chatID int64, err error) {
	if q == nil || len(q.ID) == 0 {
		return
	}

	if len(q.Queue) == 0 {
		err = errors.New("queue is empty")
		return
	}

	chatID = q.Queue[0]

	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayRemove(chatID)},
	})
	if err != nil {
		return
	}

	copy(q.Queue[0:], q.Queue[1:])
	q.Queue = q.Queue[:q.Len()-1]
	return
}
