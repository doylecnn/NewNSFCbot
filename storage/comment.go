package storage

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
)

// Comment 留言
type Comment struct {
	Username string    `firestore:"name"`
	UserID   int       `firestore:"uid"`
	Comment  string    `firestore:"comment"`
	Time     time.Time `firestore:"time"`
}

// CreateNewComment user leave new comment
func CreateNewComment(ctx context.Context, comment Comment) (err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()

	_, err = client.Collection("comments").NewDoc().Set(ctx, comment)
	return
}
