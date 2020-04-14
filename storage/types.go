package storage

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	fuzzy "github.com/doylecnn/go-fuzzywuzzy"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ProjectID gae project id
	ProjectID string
)

// User Telegram User
type User struct {
	Path            string      `firestore:"-"`
	ID              int         `firestore:"id"`
	Name            string      `firestore:"name"`
	NameInsensitive string      `firestore:"name_insensitive"`
	NSAccounts      []NSAccount `firestore:"ns_accounts,omitempty"`
	Island          *Island     `firestore:"-"`
	GroupIDs        []int64     `firestore:"groupids,omitempty"`
}

// Set new user
func (u User) Set(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Collection("users").Doc(fmt.Sprintf("%d", u.ID)).Set(ctx, u)
	if err != nil {
		logrus.Warnf("Failed adding user: %v", err)
	}
	return
}

// Update user info
func (u User) Update(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Doc(u.Path).Set(ctx, u)
	if err != nil {
		logrus.Warnf("Failed update user: %v", err)
	}
	return
}

// Delete User Info
func (u User) Delete(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	docRef := client.Doc(u.Path)
	if docRef != nil {
		games := docRef.Collection("games")
		if games != nil {
			priceHistory := games.Doc("animal_crossing").Collection("price_history")
			if err = DeleteCollection(ctx, client, priceHistory, 10); err != nil {
				logrus.Warnf("Failed delete collection price_history: %v", err)
			}
			if err = DeleteCollection(ctx, client, games, 10); err != nil {
				logrus.Warnf("Failed delete collection games: %v", err)
			}
		}
		if _, err = docRef.Delete(ctx); err != nil {
			logrus.Warnf("Failed delete doc user: %v", err)
		}
	}
	return
}

// DeleteNSAccount delete NSAccount
func (u User) DeleteNSAccount(ctx context.Context, account NSAccount) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	co := client.Doc(u.Path)
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "ns_accounts", Value: firestore.ArrayRemove(account)},
	})
	return
}

// GetUser by userid
func GetUser(ctx context.Context, userID int, groupID int64) (user *User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	dsnap, err := client.Doc(fmt.Sprintf("users/%d", userID)).Get(ctx)
	if err != nil {
		if err != nil && status.Code(err) != codes.NotFound {
			logrus.Warnf("Failed when get user: %v", err)
		}
		return nil, err
	}
	user = &User{}
	if err = dsnap.DataTo(user); err != nil {
		return nil, err
	}
	if groupID == 0 {
		return user, nil
	}
	for _, gid := range user.GroupIDs {
		if gid == groupID {
			user.Path = fmt.Sprintf("users/%d", user.ID)
			return user, nil
		}
	}
	return
}

// GetAllUsers get all users
func GetAllUsers(ctx context.Context) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	iter := client.Collection("users").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		u := &User{}
		if err = doc.DataTo(u); err != nil {
			logrus.Warn(err)
			return nil, err
		}
		if u != nil {
			u.Path = fmt.Sprintf("users/%d", u.ID)
			users = append(users, u)
		}
	}
	return users, nil
}

// GetUsersByName get users by username
func GetUsersByName(ctx context.Context, username string, groupID int64) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	users = []*User{}
	iter := client.Collection("users").Where("name_insensitive", "==", strings.ToLower(username)).Where("groupids", "array-contains", groupID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		u := &User{}
		if err = doc.DataTo(u); err != nil {
			logrus.WithError(err).Warn("GetUsersByName")
			continue
		}
		if u != nil {
			u.Path = fmt.Sprintf("users/%d", u.ID)
			users = append(users, u)
		}
	}
	return users, nil
}

// GetUsersByNSAccountName get users by Nintendo Account name
func GetUsersByNSAccountName(ctx context.Context, username string, groupID int64) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	users = []*User{}
	iter := client.Collection("users").Where("groupids", "array-contains", groupID).Documents(ctx)
	for {
		userDocSnap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if userDocSnap.Exists() {
			u := &User{}
			if err = userDocSnap.DataTo(u); err != nil {
				logrus.WithError(err).Warn("GetUsersByNSAccountName")
				continue
			}
			if u != nil {
				for _, a := range u.NSAccounts {
					if a.NameInsensitive == strings.ToLower(username) {
						u.Path = fmt.Sprintf("users/%d", u.ID)
						users = append(users, u)
					}
				}
			}
		}
	}
	return users, nil
}

// RemoveGroupIDFromUserGroupIDs remove groupid from user's groupids
func RemoveGroupIDFromUserGroupIDs(ctx context.Context, userID int, groupID int64) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	co := client.Doc(fmt.Sprintf("users/%d", userID))
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "groupids", Value: firestore.ArrayRemove(groupID)},
	})
	return
}

// AddGroupIDToUserGroupIDs add groupid to user's groupids
func AddGroupIDToUserGroupIDs(ctx context.Context, userID int, groupID int64) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	co := client.Doc(fmt.Sprintf("users/%d", userID))
	_, err = co.Update(ctx, []firestore.Update{
		{Path: "groupids", Value: firestore.ArrayUnion(groupID)},
	})
	return
}

// GetAnimalCrossingIsland get island name
func (u *User) GetAnimalCrossingIsland(ctx context.Context) (island *Island, err error) {
	if u == nil {
		return
	}

	island, err = GetAnimalCrossingIslandByUserID(ctx, u.ID)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			logrus.Warnf("failed when get island: %v", err)
		}
		return nil, err
	}
	return
}

// GetUsersByAnimalCrossingIslandName get users by island name
func GetUsersByAnimalCrossingIslandName(ctx context.Context, name string, groupID int64) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	iter := client.Collection("users").Where("groupids", "array-contains", groupID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			if status.Code(err) == codes.NotFound {
				logrus.Debug("not found user in group")
			}
			return nil, err
		}
		if !doc.Exists() {
			continue
		}
		if islandDoc, err := doc.Ref.Collection("games").Doc("animal_crossing").Get(ctx); err == nil && islandDoc.Exists() {
			var island Island
			if err = islandDoc.DataTo(&island); err != nil {
				logrus.WithError(err).Error("error when DataTo island")
				continue
			}

			name = strings.ToLower(name)
			if strings.HasSuffix(name, "岛") {
				r := []rune(name)
				l := len(r)
				if l > 1 {
					name = string(r[:l-1])
				}
			}
			if island.NameInsensitive == name || island.NameInsensitive == name+"岛" {
				u := &User{}
				if err = doc.DataTo(u); err != nil {
					logrus.WithError(err).Error("error when DataTo user")
					continue
				}
				if u != nil {
					island.Path = fmt.Sprintf("users/%d/games/animal_crossing", u.ID)
					island.LastPrice.Timezone = island.Timezone
					u.Island = &island
					u.Path = fmt.Sprintf("users/%d", u.ID)
					users = append(users, u)
				}
			}
		}
	}
	return users, nil
}

// GetUsersByAnimalCrossingIslandOwnerName get users by island owner name
func GetUsersByAnimalCrossingIslandOwnerName(ctx context.Context, name string, groupID int64) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	iter := client.Collection("users").Where("groupids", "array-contains", groupID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			if status.Code(err) == codes.NotFound {
				logrus.Debug("not found user in group")
			}
			return nil, err
		}
		if !doc.Exists() {
			continue
		}
		if islandDoc, err := doc.Ref.Collection("games").Doc("animal_crossing").Get(ctx); err == nil && islandDoc.Exists() {
			var island Island
			if err = islandDoc.DataTo(&island); err != nil {
				logrus.WithError(err).Error("error when DataTo island")
				continue
			}
			if island.OwnerInsensitive == strings.ToLower(name) {
				u := &User{}
				if err = doc.DataTo(u); err != nil {
					logrus.WithError(err).Error("error when DataTo user")
					continue
				}
				if u != nil {
					island.Path = fmt.Sprintf("users/%d/games/animal_crossing", u.ID)
					island.LastPrice.Timezone = island.Timezone
					u.Island = &island
					u.Path = fmt.Sprintf("users/%d", u.ID)
					users = append(users, u)
				}
			}
		}
	}
	return users, nil
}

// GetUsersByAnimalCrossingIslandInfo get users by island open info
func GetUsersByAnimalCrossingIslandInfo(ctx context.Context, info string, groupID int64) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	iter := client.Collection("users").Where("groupids", "array-contains", groupID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			if status.Code(err) == codes.NotFound {
				logrus.Debug("not found user in group")
			}
			return nil, err
		}
		if !doc.Exists() {
			continue
		}
		if islandDoc, err := doc.Ref.Collection("games").Doc("animal_crossing").Get(ctx); err == nil && islandDoc.Exists() {
			var island Island
			if err = islandDoc.DataTo(&island); err != nil {
				logrus.WithError(err).Error("error when DataTo island")
				continue
			}
			if len(island.BaseInfo) == 0 && len(island.Fruits) > 0 {
				island.BaseInfo = strings.Join(island.Fruits, ", ")
				island.Fruits = nil
			}
			if len(island.Info) > 0 && fuzzy.PartialRatio(island.Info, info) > 80 ||
				len(island.BaseInfo) > 0 && fuzzy.PartialRatio(island.BaseInfo, info) > 80 {
				u := &User{}
				if err = doc.DataTo(u); err != nil {
					logrus.WithError(err).Error("error when DataTo user")
					continue
				}
				if u != nil {
					island.Path = fmt.Sprintf("users/%d/games/animal_crossing", u.ID)
					island.LastPrice.Timezone = island.Timezone
					u.Island = &island
					u.Path = fmt.Sprintf("users/%d", u.ID)
					users = append(users, u)
				}
			}
		}
	}
	return users, nil
}

// GetGroupUsers get group users
func GetGroupUsers(ctx context.Context, groupID int64) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	iter := client.Collection("users").Where("groupids", "array-contains", groupID).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		u := &User{}
		if err = doc.DataTo(u); err != nil {
			return nil, err
		}
		if u != nil {
			u.Path = fmt.Sprintf("users/%d", u.ID)
			users = append(users, u)
		}
	}
	return users, nil
}

// Group telegram group info
type Group struct {
	ID    int64  `firestore:"id"`
	Type  string `firestore:"type"`
	Title string `firestore:"title"`
}

// GetAllGroups get all groups
func GetAllGroups(ctx context.Context) (groups []*Group, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	iter := client.Collection("groups").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		g := &Group{}
		if err = doc.DataTo(g); err != nil {
			logrus.Warn(err)
			return nil, err
		}
		if g != nil {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

// Set group info
func (g Group) Set(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	groupID := fmt.Sprintf("%d", g.ID)
	_, err = client.Collection("groups").Doc(groupID).Set(ctx, g)
	if err != nil {
		logrus.Warnf("Failed adding group info: %v", err)
	}
	return
}

// Update group info
func (g Group) Update(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Doc(fmt.Sprintf("groups/%d", g.ID)).Set(ctx, g)
	if err != nil {
		logrus.Warnf("Failed update group info: %v", err)
	}
	return
}

// GetGroup by group id
func GetGroup(ctx context.Context, groupID int64) (group Group, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	dsnap, err := client.Doc(fmt.Sprintf("groups/%d", groupID)).Get(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		if status.Code(err) != codes.NotFound {
			logrus.Warnf("Failed when get group: %v", err)
		}
		return
	}
	if err = dsnap.DataTo(&group); err != nil {
		return
	}
	return
}

// DeleteCollection help delete whole collection
func DeleteCollection(ctx context.Context, client *firestore.Client,
	ref *firestore.CollectionRef, batchSize int) error {
	if ref == nil {
		return nil
	}
	for {
		// Get a batch of documents
		iter := ref.Limit(batchSize).Documents(ctx)
		numDeleted := 0

		// Iterate through the documents, adding
		// a delete operation for each one to a
		// WriteBatch.
		batch := client.Batch()
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return fmt.Errorf("error when delete collection %s doc: %s. err: %w", ref.Path, doc.Ref.Path, err)
			}

			batch.Delete(doc.Ref)
			numDeleted++
		}

		// If there are no documents to delete,
		// the process is over.
		if numDeleted == 0 {
			return nil
		}

		_, err := batch.Commit(ctx)
		if err != nil {
			return fmt.Errorf("error when delete collection commit %s. err: %w", ref.Path, err)
		}
	}
}
