package storage

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type guest struct {
	UID  int64  `firestore:"UID"`
	Name string `firestore:"Name"`
}

// OnboardQueue 登岛队列
type OnboardQueue struct {
	ID            string  `firestore:"-"`
	IsAuto        bool    `firestore:"IsAuto"`
	Name          string  `firestore:"Name"`
	OwnerID       int64   `firestore:"OwnerID"`
	Owner         string  `firestore:"Owner"`
	IslandInfo    string  `firestore:"IslandInfo"`
	MaxGuestCount int     `firestore:"MaxGuestCount"`
	Password      string  `firestore:"Password"`
	Queue         []guest `firestore:"queue"`
	UIDs          []int64 `firestore:"uids"`   //private chat id
	Landed        []guest `firestore:"landed"` //landed
	Dismissed     bool    `firestore:"Dismissed"`
}

// GetJoinedQueue return joined onboard queue
func GetJoinedQueue(ctx context.Context, uid int64) (queue []*OnboardQueue, err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()
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
			logger.Warn().Err(err).Msg("GetJoinedQueue")
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
	return len(q.Queue)
}

// LandedLen return length of Landed
func (q *OnboardQueue) LandedLen() int {
	if q == nil {
		return 0
	}
	return len(q.Landed)
}

// GetPosition GetPosition
func (q *OnboardQueue) GetPosition(uid int64) (int, error) {
	for i, id := range q.UIDs {
		if uid == id {
			return i + 1, nil
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
	for _, p := range q.Landed {
		if p.UID == uid {
			return errors.New("already land island")
		}
	}
	co := client.Doc("onboardQueues/" + q.ID)
	var p = guest{UID: uid, Name: username}
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
	var deleteItem guest
	for _, p := range q.Queue {
		if p.UID == uid {
			deleteItem = p
			exists = true
			break
		}
	}
	if !exists {
		for _, p := range q.Landed {
			if p.UID == uid {
				deleteItem = p
				exists = true
				break
			}
		}
	}
	if !exists {
		return errors.New("not join in this queue")
	}
	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayRemove(deleteItem)},
		{Path: "uids", Value: firestore.ArrayRemove(uid)},
		{Path: "landed", Value: firestore.ArrayRemove(deleteItem)},
	})
	if err != nil {
		return
	}
	if len(q.Queue) > 1 {
		copy(q.Queue[0:], q.Queue[1:])
		q.Queue = q.Queue[:len(q.Queue)-1]
	} else {
		q.Queue = []guest{}
	}
	if len(q.Landed) > 1 {
		copy(q.Landed[0:], q.Landed[1:])
		q.Landed = q.Landed[:len(q.Landed)-1]
	} else {
		q.Landed = []guest{}
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

	g := q.Queue[0]
	chatID = g.UID

	co := client.Doc("onboardQueues/" + q.ID)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "queue", Value: firestore.ArrayRemove(g)},
		{Path: "uids", Value: firestore.ArrayRemove(g.UID)},
		{Path: "landed", Value: firestore.ArrayUnion(g)},
	})
	if err != nil {
		return
	}

	q.Landed = append(q.Landed, g)
	copy(q.Queue[0:], q.Queue[1:])
	q.Queue = q.Queue[:len(q.Queue)-1]

	return
}
