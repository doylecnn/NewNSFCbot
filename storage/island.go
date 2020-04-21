package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//Timezone timezone
type Timezone int

// Location return *time.Location
func (t Timezone) Location() *time.Location {
	if t != 0 {
		return time.FixedZone(t.String(), int(t))
	}
	return time.UTC
}

func (t Timezone) String() string {
	h := t / 3600
	sign := "-"
	if h >= 0 {
		sign = "+"
	}
	m := (t - h*3600) % 60
	if sign == "-" {
		h = -h
		m = -m
	}
	return fmt.Sprintf("%s%02d%02d", sign, h, m)
}

// Island in AnimalCrossing
type Island struct {
	Path             string          `firestore:"-"`
	Name             string          `firestore:"name"`
	NameInsensitive  string          `firestore:"name_insensitive,omitempty"`
	Hemisphere       int             `firestore:"hemisphere"`
	AirportIsOpen    bool            `firestore:"AirportIsOpen"`
	OpenTime         time.Time       `firestore:"OpenTime,omitempty"`
	BaseInfo         string          `firestore:"BaseInfo"`
	Info             string          `firestore:"Info"`
	OnBoardQueueID   string          `firestore:"OnBoardQueueID"`
	Timezone         Timezone        `filestore:"timezone"`
	Fruits           []string        `firestore:"Fruits,omitempty"`
	LastPrice        PriceHistory    `firestore:"LastPrice,omitempty"`
	Owner            string          `firestore:"owner,omitempty"`
	OwnerInsensitive string          `firestore:"owner_insensitive,omitempty"`
	WeekPriceHistory []*PriceHistory `firestore:"-"`
}

// GetAnimalCrossingIslandByUserID get island by user id
func GetAnimalCrossingIslandByUserID(ctx context.Context, uid int) (island *Island, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	var islandDocPath = fmt.Sprintf("users/%d/games/animal_crossing", uid)
	dsnap, err := client.Doc(islandDocPath).Get(ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			logrus.Warnf("failed when get island: %v", err)
		}
		return nil, err
	}
	island = &Island{}
	if err = dsnap.DataTo(island); err != nil {
		return
	}
	island.Path = islandDocPath
	needUpdate := false
	if math.Abs(float64(island.Timezone)) > 1000000000.0 {
		island.Timezone /= 1000000000
		needUpdate = true
	}
	if math.Abs(float64(island.LastPrice.Timezone)) > 1000000000.0 {
		island.LastPrice.Timezone /= 1000000000
		needUpdate = true
	}
	if island.AirportIsOpen {
		locOpenTime := island.OpenTime.In(island.Timezone.Location())
		locNow := time.Now().In(island.Timezone.Location())
		if locNow.Hour() >= 5 && (locOpenTime.Hour() >= 0 && locOpenTime.Hour() < 5 ||
			locNow.Day()-locOpenTime.Day() >= 1) {
			island.Close(ctx)
		}
	}
	if needUpdate {
		island.Update(ctx)
	}
	return
}

// Set island info
func (i Island) Set(ctx context.Context, uid int) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	_, err = client.Collection(fmt.Sprintf("users/%d/games", uid)).Doc("animal_crossing").Set(ctx, i)
	return
}

// Update island info
func (i Island) Update(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	logrus.WithFields(logrus.Fields{
		"name":     i.Name,
		"owner":    i.Owner,
		"timezone": i.Timezone,
		"path":     i.Path,
	}).Debug("update island")
	_, err = client.Doc(i.Path).Set(ctx, i)
	return
}

// Close island
func (i *Island) Close(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	if len(i.OnBoardQueueID) > 0 {
		_, err = i.ClearOldOnboardQueue(ctx)
		if err != nil {
			return
		}
	}
	i.AirportIsOpen = false
	i.Info = ""
	return i.Update(ctx)
}

// CreateOnboardQueue create onboard island queue
func (i *Island) CreateOnboardQueue(ctx context.Context, uid int64, owner, password, specialInfo string, maxGuestCount int) (queue *OnboardQueue, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	if len(specialInfo) >= 0 {
		i.Info = specialInfo
	}
	queue = &OnboardQueue{Name: i.Name, OwnerID: uid, Owner: owner, Password: password, IslandInfo: i.ShortInfo(), MaxGuestCount: maxGuestCount}
	err = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		ref := client.Collection("onboardQueues").NewDoc()
		queue.ID = ref.ID
		if err = tx.Create(ref, queue); err != nil {
			return err
		}
		islandRef := client.Doc(i.Path)
		return tx.Set(islandRef, map[string]interface{}{
			"OnBoardQueueID": queue.ID,
			"OpenTime":       time.Now(),
			"AirportIsOpen":  true,
		}, firestore.MergeAll)
	})
	if err != nil {
		logrus.WithError(err).Info("An error has occurred when CreateOnboardQueue")
	}
	return
}

// GetOnboardQueue return a exists OnboardQueue
func (i *Island) GetOnboardQueue(ctx context.Context) (queue *OnboardQueue, err error) {
	if len(i.OnBoardQueueID) == 0 {
		return nil, errors.New("NotFound")
	}
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()
	return GetOnboardQueue(ctx, client, i.OnBoardQueueID)
}

// ClearOldOnboardQueue clean old onboard island queue
func (i *Island) ClearOldOnboardQueue(ctx context.Context) (queue *OnboardQueue, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	queue = &OnboardQueue{}

	err = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if len(i.OnBoardQueueID) > 0 {
			islandRef := client.Doc(i.Path)
			ref := client.Doc("onboardQueues/" + i.OnBoardQueueID)
			doc, err := tx.Get(ref)
			if err != nil {
				if status.Code(err) == codes.NotFound {
					return tx.Set(islandRef, map[string]interface{}{
						"OnBoardQueueID": "",
					}, firestore.MergeAll)
				}
				return nil
			}
			err = tx.Set(islandRef, map[string]interface{}{
				"OnBoardQueueID": "",
			}, firestore.MergeAll)
			if err != nil {
				return err
			}
			err = doc.DataTo(queue)
			if err != nil {
				return err
			}
			queue.Dismissed = true
			return tx.Delete(ref)
		}
		return err
	})
	if err != nil {
		logrus.WithError(err).Info("An error has occurred when ClearOldOnboardQueue")
	}
	return
}

// ShortInfo short island info
func (i Island) ShortInfo() string {
	airportstatus := "现正开放"
	var hemisphere string
	if i.Hemisphere == 0 {
		hemisphere = "北"
	} else {
		hemisphere = "南"
	}
	if !strings.HasSuffix(i.Name, "岛") {
		i.Name += "岛"
	}
	if len(i.BaseInfo) == 0 {
		i.BaseInfo = strings.Join(i.Fruits, ", ")
		if err := i.Update(context.Background()); err != nil {
			logrus.WithError(err).Error()
		}
	}
	var text string = fmt.Sprintf("位于%s半球%s时区的岛屿：%s, 岛民代表：%s。 %s\n基本信息：%s\n\n", hemisphere, i.Timezone.String(), i.Name, i.Owner, airportstatus, i.BaseInfo)
	if i.AirportIsOpen {
		if len(i.Info) > 0 {
			text += "本回开放特色信息：" + i.Info
		}
	}
	return text
}

func (i Island) String() string {
	var airportstatus string
	if i.AirportIsOpen {
		airportstatus = fmt.Sprintf("现正开放：已开放 %d 分钟", int(time.Since(i.OpenTime).Minutes()))
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
	if len(i.BaseInfo) == 0 {
		i.BaseInfo = strings.Join(i.Fruits, ", ")
		if err := i.Update(context.Background()); err != nil {
			logrus.WithError(err).Error()
		}
	}
	var text string = fmt.Sprintf("位于%s半球%s时区的岛屿：%s, 岛民代表：%s。 %s\n基本信息：%s\n\n", hemisphere, i.Timezone.String(), i.Name, i.Owner, airportstatus, i.BaseInfo)
	if i.AirportIsOpen {
		if len(i.OnBoardQueueID) > 0 {
			text += "本回通过密码才能访问：需要排队"
		}
		if len(i.Info) > 0 {
			text += "本回开放特色信息：" + i.Info
		}
	}
	return text
}

// PriceHistory 大头菜 price history
type PriceHistory struct {
	Path     string    `firestore:"-"`
	Date     time.Time `firestore:"Date"`
	Price    int       `firestore:"Price"`
	Timezone Timezone  `firestore:"Timezone"`
}

// Set price history
func (p PriceHistory) Set(ctx context.Context, uid int) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	_, err = client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid)).Doc(fmt.Sprintf("%d", p.Date.Unix())).Set(ctx, p)
	if err != nil {
		err = fmt.Errorf("error when set price history%w", err)
	}
	return
}

// Update price history
func (p PriceHistory) Update(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	_, err = client.Doc(p.Path).Set(ctx, p)
	return
}

// Delete price history
func (p PriceHistory) Delete(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return
	}
	defer client.Close()

	_, err = client.Doc(p.Path).Delete(ctx)
	return
}

//LocationDateTime get location datetime
func (p PriceHistory) LocationDateTime() (datetime time.Time) {
	var loc *time.Location
	loc = p.Timezone.Location()
	datetime = p.Date.In(loc)
	return
}

// UpdateDTCPrice 更新 大头菜 菜价
func UpdateDTCPrice(ctx context.Context, uid, price int) (err error) {
	island, err := GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		logrus.WithError(err).Error("GetAnimalCrossingIslandByUserID")
		return
	}
	lp, err := GetLastPriceHistory(ctx, uid, island.LastPrice.Date)
	if err != nil {
		if err.Error() != "NotFound" && status.Code(err) != codes.NotFound {
			logrus.WithError(err).Error("GetLastPriceHistory")
			return
		}
	}
	now := time.Now()
	if island.Timezone != 0 {
		islandLoc := island.Timezone.Location()
		loc := now.In(islandLoc)
		if loc.Weekday() == 0 {
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 5, 0, 0, 0, islandLoc).UTC()
		} else if loc.Hour() >= 8 && loc.Hour() < 12 {
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 8, 0, 0, 0, islandLoc).UTC()
		} else if loc.Hour() < 8 || loc.Hour() >= 12 {
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 12, 0, 0, 0, islandLoc).UTC()
		}
	}
	priceHistory := PriceHistory{Date: now, Price: price, Timezone: island.Timezone}
	island.LastPrice = priceHistory
	if err = island.Update(ctx); err != nil {
		logrus.WithError(err).Error("update island last price")
		return
	}
	if lp != nil {
		lpd := lp.LocationDateTime()
		pd := priceHistory.LocationDateTime()
		if lpd.Day() == pd.Day() &&
			((lpd.Weekday() == 0 && pd.Weekday() == 0) ||
				(lpd.Weekday() > 0 && pd.Weekday() > 0 &&
					(lpd.Hour() >= 8 && lpd.Hour() < 12 && pd.Hour() == 8) ||
					(lpd.Hour() >= 12 && lpd.Hour() < 8 && pd.Hour() == 12))) {
			if err = lp.Delete(ctx); err != nil {
				logrus.WithError(err).Error("Delete old price")
				return
			}
		}
	}
	return priceHistory.Set(ctx, uid)
}

// GetLastPriceHistory get price history
func GetLastPriceHistory(ctx context.Context, uid int, lasttime time.Time) (priceHistory *PriceHistory, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	priceHistory = &PriceHistory{}
	phPath := fmt.Sprintf("users/%d/games/animal_crossing/price_history/%d", uid, lasttime.Unix())
	dsnap, err := client.Doc(phPath).Get(ctx)
	if err == nil {
		if err = dsnap.DataTo(priceHistory); err != nil {
			return nil, err
		}
		priceHistory.Path = phPath
	} else {
		if status.Code(err) == codes.NotFound {
			phs, err := getPriceHistory(ctx, client, uid, "Date", firestore.Desc, 1)
			if err != nil {
				return nil, err
			}
			if len(phs) > 0 && phs[0] != nil {
				priceHistory = phs[0]
			}
		} else {
			return nil, err
		}
	}
	if priceHistory == nil {
		return nil, errors.New("NotFound")
	}
	return priceHistory, nil
}

// GetPriceHistory get price history
func GetPriceHistory(ctx context.Context, uid int) (priceHistory []*PriceHistory, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return getPriceHistory(ctx, client, uid, "Date", firestore.Asc, 0)
}

func getPriceHistory(ctx context.Context, client *firestore.Client, uid int, path string, dir firestore.Direction, limit int) (priceHistory []*PriceHistory, err error) {
	query := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid)).OrderBy("Date", dir)
	var iter *firestore.DocumentIterator
	if limit > 0 {
		iter = query.Limit(limit).Documents(ctx)
	} else {
		iter = query.Documents(ctx)
	}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var price *PriceHistory = &PriceHistory{}
		if err = doc.DataTo(price); err != nil {
			logrus.Warn(err)
			return nil, err
		}
		price.Path = fmt.Sprintf("users/%d/games/animal_crossing/price_history/%d", uid, price.Date.Unix())
		priceHistory = append(priceHistory, price)
	}
	return priceHistory, nil
}

// GetWeeklyDTCPriceHistory 获得当前周自周日起的价格。周日是买入价
func GetWeeklyDTCPriceHistory(ctx context.Context, uid int, startDate, endDate time.Time) (priceHistory []*PriceHistory, err error) {
	client, err := firestore.NewClient(ctx, ProjectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	iter := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid)).Where("Date", ">=", startDate).Where("Date", "<", endDate).OrderBy("Date", firestore.Asc).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var price *PriceHistory = &PriceHistory{}
		if err = doc.DataTo(price); err != nil {
			logrus.Warn(err)
			return nil, err
		}
		price.Path = fmt.Sprintf("users/%d/games/animal_crossing/price_history/%d", uid, price.Date.Unix())
		priceHistory = append(priceHistory, price)
	}
	return priceHistory, nil
}
