package storage

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type persion struct {
	UID  int64  `firestore:"UID"`
	Name string `firestore:"Name"`
}

// OnboardQueue 登岛队列
type OnboardQueue struct {
	ID             string    `firestore:"-"`
	Name           string    `firestore:"Name"`
	OwnerID        int64     `firestore:"OwnerID"`
	Owner          string    `firestore:"Owner"`
	IslandInfo     string    `firestore:"IslandInfo"`
	CurrentOnBoard string    `firestore:"CurrentOnBoard"`
	MaxGuestCount  int       `firestore:"MaxGuestCount"`
	Password       string    `firestore:"Password"`
	Queue          []persion `firestore:"queue"`
	UIDs           []int64   `firestore:"uids"` //private chat id
	Dismissed      bool      `firestore:"Dismissed"`
}

// GetJoinedQueue return joined onboard queue
func GetJoinedQueue(ctx context.Context, uid int64) (queue []*OnboardQueue, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	iter := client.Collection("onboardQueues").Where("uids", "array-contains", uid).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		q := &OnboardQueue{}
		if err = doc.DataTo(q); err != nil {
			Logger.WithError(err).Warn("GetJoinedQueue")
			continue
		}
		if q != nil {
			q.ID = doc.Ref.ID
			queue = append(queue, q)
		}
	}
	return queue, nil
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
	if q == nil {
		return 0
	}
	return len(q.UIDs)
}

// GetPosition GetPosition
func (q *OnboardQueue) GetPosition(uid int64) (int, error) {
	for i, id := range q.UIDs {
		if uid == id {
			return i, nil
		}
	}
	return -1, errors.New("NotFound")
}

// Append chatID into OnboardQueue
func (q *OnboardQueue) Append(ctx context.Context, client *firestore.Client, uid int64, username string) (err error) {
	if q == nil || len(q.ID) == 0 {
		return
	}
	if q.Dismissed {
		return errors.New("queue has been dismissed")
	}
	for _, p := range q.Queue {
		if p.UID == uid {
			return errors.New("already in this queue")
		}
	}
	co := client.Doc("onboardQueues/" + q.ID)
	var p = persion{UID: uid, Name: username}
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayUnion(p)},
		{Path: "uids", Value: firestore.ArrayUnion(uid)},
	})
	if err != nil {
		return
	}
	q.Queue = append(q.Queue, p)
	q.UIDs = append(q.UIDs, uid)
	return
}

// Remove chatID into OnboardQueue
func (q *OnboardQueue) Remove(ctx context.Context, client *firestore.Client, uid int64) (err error) {
	if q == nil || len(q.ID) == 0 {
		return
	}
	if q.Dismissed {
		return errors.New("queue has been dismissed")
	}
	var exists = false
	var deleteItem persion
	for _, p := range q.Queue {
		if p.UID == uid {
			deleteItem = p
			exists = true
			break
		}
	}
	if !exists {
		return errors.New("not join in this queue")
	}
	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayRemove(deleteItem)},
		{Path: "uids", Value: firestore.ArrayUnion(uid)},
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

	p := q.Queue[0]
	chatID = p.UID

	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayRemove(p)},
		{Path: "uids", Value: firestore.ArrayRemove(p.UID)},
	})
	if err != nil {
		return
	}

	copy(q.Queue[0:], q.Queue[1:])
	q.Queue = q.Queue[:len(q.Queue)-1]

	return
}
