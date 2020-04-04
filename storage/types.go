package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
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
	ID         int         `firestore:"id"`
	Name       string      `firestore:"name"`
	NSAccounts []NSAccount `firestore:"ns_accounts,omitempty"`
	Island     Island      `firestore:"-"`
	GroupIDs   []int64     `firestore:"groupids,omitempty"`
}

// Create new user
func (u User) Create(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	userID := fmt.Sprintf("%d", u.ID)
	_, err = client.Collection("users").Doc(userID).Set(ctx, u)
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

	_, err = client.Doc(fmt.Sprintf("users/%d", u.ID)).Set(ctx, u)
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

	docRef := client.Doc(fmt.Sprintf("users/%d", u.ID))
	games := docRef.Collection("games")
	priceHistory := games.Doc("animal_crossing").Collection("price_history")
	if err = deleteCollection(ctx, client, priceHistory, 10); err != nil {
		logrus.Warnf("Failed delete collection price_history: %v", err)
	}
	if err = deleteCollection(ctx, client, games, 10); err != nil {
		logrus.Warnf("Failed delete collection games: %v", err)
	}
	if _, err = docRef.Delete(ctx); err != nil {
		logrus.Warnf("Failed delete doc user: %v", err)
	}
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
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.Warnf("Failed when get user: %v", err)
		return nil, err
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		logrus.Warnf("Not found userID: %d", userID)
		return nil, fmt.Errorf("Not found userID: %d", userID)
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
			return user, nil
		}
	}
	return nil, fmt.Errorf("Not found userID: %d", userID)
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
	iter := client.Collection("users").Where("name", "==", username).Where("groupids", "array-contains", groupID).Documents(ctx)
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
				logrus.Warn(err)
				return nil, err
			}
			if u != nil {
				for _, a := range u.NSAccounts {
					if a.Name == username {
						users = append(users, u)
						break
					}
				}
				break
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

// AddAnimalCrossingIsland add island name
func (u *User) AddAnimalCrossingIsland(ctx context.Context, island Island) (err error) {
	if u == nil {
		return
	}

	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	colref := client.Collection(fmt.Sprintf("users/%d/games", u.ID))
	_, err = colref.Doc("animal_crossing").Set(ctx, island)
	return
}

// SetAirportStatus Set Airport Status
func (u *User) SetAirportStatus(ctx context.Context, island Island) (err error) {
	if u == nil {
		return
	}

	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	colref := client.Collection(fmt.Sprintf("users/%d/games", u.ID))
	_, err = colref.Doc("animal_crossing").Set(ctx, island)
	return
}

// GetAnimalCrossingIsland get island name
func (u *User) GetAnimalCrossingIsland(ctx context.Context) (island *Island, err error) {
	if u == nil {
		return
	}

	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	dsnap, err := client.Doc(fmt.Sprintf("users/%d/games/animal_crossing", u.ID)).Get(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.Warnf("failed when get island: %v", err)
		return nil, err
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		return nil, fmt.Errorf("Not found island of userID: %d", u.ID)
	}
	island = &Island{}
	err = dsnap.DataTo(island)
	return
}

// UpdateDTCPrice 更新 大头菜 菜价
func UpdateDTCPrice(ctx context.Context, uid, price int) error {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()
	dsnap, err := client.Doc(fmt.Sprintf("users/%d/games/animal_crossing", uid)).Get(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.Warnf("failed when get island: %v", err)
		return err
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		return errors.New("Not found game: animal_crossing")
	}
	island := Island{}
	err = dsnap.DataTo(&island)
	if err != nil {
		return err
	}
	island.LastPrice.Price = price
	island.LastPrice.Date = time.Now()
	client.Doc(fmt.Sprintf("users/%d/games/animal_crossing", uid)).Set(ctx, island)
	var now = time.Now()
	cref := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid))
	_, err = cref.Doc(fmt.Sprintf("%d", now.Unix())).Set(ctx, island.LastPrice)
	return err
}

// GetPriceHistory get price history
func GetPriceHistory(ctx context.Context, uid int) (priceHistory []PriceHistory, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	iter := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid)).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var price PriceHistory
		if err = doc.DataTo(&price); err != nil {
			logrus.Warn(err)
			return nil, err
		}
		priceHistory = append(priceHistory, price)
	}
	return priceHistory, nil
}

// Island in AnimalCrossing
type Island struct {
	Name          string       `firestore:"name"`
	Hemisphere    int          `firestore:"hemisphere"`
	AirportIsOpen bool         `firestore:"AirportIsOpen"`
	Info          string       `firestore:"Info"`
	Fruits        []string     `firestore:"Fruits"`
	LastPrice     PriceHistory `firestore:"LastPrice"`
	Owner         string       `firestore:"owner,omitempty"`
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
			islandDoc.DataTo(&island)
			if strings.HasSuffix(name, "岛") {
				r := []rune(name)
				l := len(r)
				if l > 1 {
					name = string(r[:l-1])
				}
			}
			if island.Name == name || island.Name == name+"岛" {
				u := &User{}
				if err = doc.DataTo(u); err != nil {
					logrus.WithError(err).Error("error when DataTo user")
					return nil, err
				}
				if u != nil {
					users = append(users, u)
					break
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
			islandDoc.DataTo(&island)
			if island.Owner == name {
				u := &User{}
				if err = doc.DataTo(u); err != nil {
					logrus.WithError(err).Error("error when DataTo user")
					return nil, err
				}
				if u != nil {
					users = append(users, u)
					break
				}
			}
		}
	}
	return users, nil
}

// PriceHistory 大头菜 price history
type PriceHistory struct {
	Date  time.Time `firestore:"Date"`
	Price int       `firestore:"Price"`
}

func (i Island) String() string {
	var airportstatus string
	if i.AirportIsOpen {
		airportstatus = "现正开放"
	} else {
		airportstatus = "现已关闭"
	}
	var hemisphere string
	if i.Hemisphere == 0 {
		hemisphere = "北"
	} else {
		hemisphere = "南"
	}
	if !strings.HasSuffix(i.Name, "岛") {
		i.Name += "岛"
	}
	var text string = fmt.Sprintf("位于%s半球的岛屿：%s, 岛民代表：%s。 %s\n岛屿有水果：%s", hemisphere, i.Name, i.Owner, airportstatus, strings.Join(i.Fruits, ", "))
	if i.AirportIsOpen && len(i.Info) > 0 {
		text += "\n\n本回开放特色信息：" + i.Info
	}
	return text
}

// NSAccount Nintendo Switch account
type NSAccount struct {
	Name string     `firestore:"name,omitempty"`
	FC   FriendCode `firestore:"friend_code,omitempty"`
}

// ParseAccountsFromString Parse FriendCode From String
func ParseAccountsFromString(msg, defaultname string) (accounts []NSAccount, err error) {
	var accountRegexp = regexp.MustCompile("^(?:(\\w+)\\s*:\\s*)?(?:[sS][wW]-?)?((?:\\d{12})|(?:\\d{4}-\\d{4}-\\d{4}))$")
	msg = strings.TrimSpace(msg) + ";"
	var substrs = strings.Split(msg, ";")
	for _, s := range substrs {
		submatchs := accountRegexp.FindAllStringSubmatch(strings.TrimSpace(s), 1)
		for _, m := range submatchs {
			code, err := strconv.ParseInt(strings.Replace(m[2], "-", "", -1), 10, 64)
			if err != nil {
				return accounts, fmt.Errorf("error: %v. wrong friend code format:%s", err, m[0])
			}
			var name string
			if len(m[1]) > 0 {
				name = m[1]
			} else {
				name = defaultname
			}
			accounts = append(accounts, NSAccount{name, FriendCode(code)})
		}
	}
	return accounts, nil
}

func (a NSAccount) String() string {
	if len(a.Name) == 0 {
		return a.FC.String()
	}
	return a.Name + ": " + a.FC.String()
}

// FriendCode is Nintendo Switch Friend Code
type FriendCode int64

func (fc FriendCode) String() string {
	c := int64(fc)
	return fmt.Sprintf("SW-%04d-%04d-%04d", c/100000000%10000, c/10000%10000, c%10000)
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

// Create group info
func (g Group) Create(ctx context.Context) (err error) {
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
		logrus.Warnf("Failed when get group: %v", err)
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		logrus.Warnf("Not found group: %d", groupID)
		err = fmt.Errorf("Not found group: %d", groupID)
	}
	if err = dsnap.DataTo(&group); err != nil {
		return
	}
	return
}

func deleteCollection(ctx context.Context, client *firestore.Client,
	ref *firestore.CollectionRef, batchSize int) error {
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
