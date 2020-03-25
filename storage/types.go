package storage

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ProjectID string
)

// User Telegram User
type User struct {
	ID         int         `firestore:"id"`
	Name       string      `firestore:"name"`
	NSAccounts []NSAccount `firestore:"-"`
	Island     Island      `firestore:"-"`
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
		log.Warnf("Failed adding aturing: %v", err)
	}
	for _, account := range u.NSAccounts {
		_, err = client.Collection("users").Doc(userID).Collection("ns_accounts").Doc(fmt.Sprintf("%d", int64(account.FC))).Set(ctx, account)
		if err != nil {
			log.Warnf("Failed adding aturing: %v", err)
			return
		}
	}
	return
}

// GetUser by userid
func GetUser(ctx context.Context, userID int) (user *User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	dsnap, err := client.Doc(fmt.Sprintf("users/%d", userID)).Get(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		log.Warnf("Failed when get user: %v", err)
		return nil, err
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		log.Warnf("Not found userID: %d", userID)
		return nil, nil
	}
	user = &User{}
	if err = dsnap.DataTo(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetAllUsers get all users
func GetAllUsers(ctx context.Context) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	users = []*User{}
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
			log.Warn(err)
			return nil, err
		}
		if u != nil {
			users = append(users, u)
		}
	}
	return users, nil
}

// GetUsersByName get users by username
func GetUsersByName(ctx context.Context, username string) (users []*User, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	users = []*User{}
	iter := client.Collection("users").Where("name", "==", username).Documents(ctx)
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
			log.Warn(err)
			return nil, err
		}
		if u != nil {
			users = append(users, u)
		}
	}
	return users, nil
}

// AddNSAccounts add NS account
func (u *User) AddNSAccounts(ctx context.Context, accounts []NSAccount) (err error) {
	if u == nil {
		return
	}
	u.NSAccounts = append(u.NSAccounts, accounts...)

	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	colref := client.Collection(fmt.Sprintf("users/%d/ns_accounts", u.ID))
	for _, account := range accounts {
		_, err = colref.Doc(fmt.Sprintf("%d", int64(account.FC))).Set(ctx, account)
		if err != nil {
			return
		}
	}
	return
}

// GetAccounts get user switch accounts
func (u *User) GetAccounts(ctx context.Context) (accounts []NSAccount, err error) {
	if u == nil {
		return
	}

	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return accounts, err
	}
	defer client.Close()

	iter := client.Collection(fmt.Sprintf("users/%d/ns_accounts", u.ID)).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var fc NSAccount
		if err = doc.DataTo(&fc); err != nil {
			log.Warn(err)
			return nil, err
		}
		log.Info(fc)
		accounts = append(accounts, fc)
	}
	u.NSAccounts = accounts[:]

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
		log.Warnf("failed when get island: %v", err)
		return nil, err
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		//log.Debugf("not found island of userID: %d", u.ID)
		return nil, nil
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
		log.Warnf("failed when get island: %v", err)
		return err
	}
	if !dsnap.Exists() || (err != nil && status.Code(err) == codes.NotFound) {
		return nil
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
			log.Warn(err)
			return nil, err
		}
		priceHistory = append(priceHistory, price)
	}
	return priceHistory, nil
}

// Island in AnimalCrossing
type Island struct {
	Name            string       `firestore:"name"`
	Hemisphere      int          `firestore:"hemisphere"`
	AirportIsOpen   bool         `firestore:"AirportIsOpen"`
	AirportPassword string       `firestore:"AirportPassword"`
	Fruits          []string     `firestore:"Fruits"`
	LastPrice       PriceHistory `firestore:"LastPrice"`
}

type PriceHistory struct {
	Date  time.Time `firestore:"Date"`
	Price int       `firestore:"Price"`
}

func (i Island) String() string {
	var airportstatus string
	if i.AirportIsOpen {
		airportstatus = "现正开放"
		if len(i.AirportPassword) != 0 {
			airportstatus += fmt.Sprintf("，密码：%s", i.AirportPassword)
		}
	} else {
		airportstatus = "现已关闭"
	}
	var hemisphere string
	if i.Hemisphere == 0 {
		hemisphere = "北"
	} else {
		hemisphere = "南"
	}
	return strings.TrimSpace(fmt.Sprintf("位于%s半球的岛屿：%s岛 %s\n岛屿有水果：%s", hemisphere, i.Name, airportstatus, strings.Join(i.Fruits, ", ")))
}

// Group Telegram Group
type Group struct {
	ID    int64
	Users []User
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
