package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
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
	Path             string        `firestore:"-"`
	Name             string        `firestore:"name"`
	NameInsensitive  string        `firestore:"name_insensitive"`
	Hemisphere       int           `firestore:"hemisphere"`
	AirportIsOpen    bool          `firestore:"AirportIsOpen"`
	OpenTime         time.Time     `firestore:"OpenTime"`
	BaseInfo         string        `firestore:"BaseInfo"`
	Info             string        `firestore:"Info"`
	OnBoardQueueID   string        `firestore:"OnBoardQueueID"`
	Timezone         Timezone      `filestore:"timezone"`
	LastPrice        TurnipPrice   `firestore:"LastPrice"`
	Owner            string        `firestore:"owner"`
	OwnerInsensitive string        `firestore:"owner_insensitive"`
	ResidentUID      int           `firestore:"resident_userid,omitempty"` // 指向真正的岛主
	WeekPriceHistory []TurnipPrice `firestore:"-"`
}

// GetAnimalCrossingIslandByUserID get island by user id
func GetAnimalCrossingIslandByUserID(ctx context.Context, uid int) (island *Island, residentUID int, err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()
	var islandDocPath = fmt.Sprintf("users/%d/games/animal_crossing", uid)
	dsnap, err := client.Doc(islandDocPath).Get(ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			logger.Error().Err(err).Int("uid", uid).Msg("failed when get island")
			return nil, 0, err
		}
	}
	island = &Island{}
	if err = dsnap.DataTo(island); err != nil {
		return
	}
	if island.ResidentUID > 0 {
		residentUID = island.ResidentUID
		islandDocPath = fmt.Sprintf("users/%d/games/animal_crossing", island.ResidentUID)
		dsnap, err = client.Doc(islandDocPath).Get(ctx)
		if err != nil {
			if status.Code(err) != codes.NotFound {
				logger.Error().Err(err).Int("uid", island.ResidentUID).Msg("failed when get island by ResidentUID")
				return nil, 0, err
			}
		}
		island = &Island{}
		if err = dsnap.DataTo(island); err != nil {
			return
		}
	}
	island.Path = islandDocPath
	needUpdate := false
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

// Update island info
func (i Island) Update(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()
	_, err = client.Doc(i.Path).Set(ctx, i)
	return
}

// Close island
func (i *Island) Close(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, projectID)
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
func (i *Island) CreateOnboardQueue(ctx context.Context, uid int64, owner, password string, maxGuestCount int) (queue *OnboardQueue, err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()

	queue = &OnboardQueue{Name: i.Name, IsAuto: maxGuestCount != 0, OwnerID: uid, Owner: owner, Password: password, IslandInfo: i.ShortInfo(), MaxGuestCount: maxGuestCount}
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
		logger.Info().Err(err).Msg("An error has occurred when CreateOnboardQueue")
	}
	return
}

// GetOnboardQueue return a exists OnboardQueue
func (i *Island) GetOnboardQueue(ctx context.Context) (queue *OnboardQueue, err error) {
	if len(i.OnBoardQueueID) == 0 {
		return nil, errors.New("NotFound")
	}
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()
	return GetOnboardQueue(ctx, client, i.OnBoardQueueID)
}

// ClearOldOnboardQueue clean old onboard island queue
func (i *Island) ClearOldOnboardQueue(ctx context.Context) (queue *OnboardQueue, err error) {
	client, err := firestore.NewClient(ctx, projectID)
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
		logger.Info().Err(err).Msg("An error has occurred when ClearOldOnboardQueue")
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

// TurnipPrice 大头菜价
type TurnipPrice struct {
	Path     string    `firestore:"-"`
	Date     time.Time `firestore:"Date"`
	Price    int       `firestore:"Price"`
	Timezone Timezone  `firestore:"Timezone"`
}

//LocationDateTime get location datetime
func (p TurnipPrice) LocationDateTime() (datetime time.Time) {
	var loc *time.Location
	loc = p.Timezone.Location()
	datetime = p.Date.In(loc)
	return
}

// UpdateDTCPrice 更新 大头菜 菜价
func UpdateDTCPrice(ctx context.Context, uid, price int) (err error) {
	island, residentUID, err := GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		logger.Warn().Err(err).Msg("GetAnimalCrossingIslandByUserID")
		return
	}
	if residentUID > 0 {
		uid = residentUID
	}
	lp, err := GetLastPriceHistory(ctx, uid, island.LastPrice.Date)
	if err != nil {
		if err.Error() != "NotFound" && status.Code(err) != codes.NotFound {
			logger.Warn().Err(err).Msg("GetLastPriceHistory")
			return
		}
	}
	now := time.Now()
	if island.Timezone != 0 {
		islandLoc := island.Timezone.Location()
		loc := now.In(islandLoc)
		if loc.Weekday() == 0 && loc.Hour() >= 5 {
			if price < 90 || price > 110 {
				err = errors.New("buy price out of range")
				return
			}
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 5, 0, 0, 0, islandLoc).UTC()
		} else if loc.Hour() >= 8 && loc.Hour() < 12 {
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 8, 0, 0, 0, islandLoc).UTC()
		} else if loc.Hour() >= 12 {
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 12, 0, 0, 0, islandLoc).UTC()
		} else if loc.Hour() < 8 {
			loc = loc.AddDate(0, 0, -1)
			now = time.Date(loc.Year(), loc.Month(), loc.Day(), 12, 0, 0, 0, islandLoc).UTC()
		}
	}
	tp := TurnipPrice{Date: now, Price: price, Timezone: island.Timezone}
	island.LastPrice = tp
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		err = fmt.Errorf("firestore.NewClient failed: %w", err)
		return
	}
	defer client.Close()
	batch := client.Batch()
	islandRef := client.Doc(island.Path)
	batch.Update(islandRef, []firestore.Update{{Path: "LastPrice", Value: tp}})
	lpd := lp.LocationDateTime()
	pd := tp.LocationDateTime()
	logger.Debug().Msg("update or create tp")
	if lpd.Day() == pd.Day() &&
		((lpd.Weekday() == 0 && pd.Weekday() == 0) ||
			(lpd.Weekday() > 0 && pd.Weekday() > 0 &&
				(lpd.Hour() == 8 && pd.Hour() == 8) ||
				(lpd.Hour() == 12 && pd.Hour() == 12))) {
		lpRef := client.Doc(lp.Path)
		batch.Update(lpRef, []firestore.Update{{Path: "Price", Value: tp.Price}})
	} else {
		newLPRef := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid)).Doc(fmt.Sprintf("%d", tp.Date.Unix()))
		batch.Create(newLPRef, tp)
	}
	_, err = batch.Commit(ctx)
	if err != nil {
		err = fmt.Errorf("batch.Commit failed: %w", err)
	}
	return
}

// GetLastPriceHistory get price history
func GetLastPriceHistory(ctx context.Context, uid int, lasttime time.Time) (tp TurnipPrice, err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()

	tp = TurnipPrice{}
	tpPath := fmt.Sprintf("users/%d/games/animal_crossing/price_history/%d", uid, lasttime.Unix())
	dsnap, err := client.Doc(tpPath).Get(ctx)
	if err == nil {
		if err = dsnap.DataTo(&tp); err != nil {
			return
		}
		tp.Path = tpPath
	} else {
		if status.Code(err) == codes.NotFound {
			var phs []TurnipPrice
			phs, err = getPriceHistory(ctx, client, uid, "Date", firestore.Desc, 1)
			if err != nil {
				return
			}
			if len(phs) > 0 {
				tp = phs[0]
			}
		} else {
			return
		}
	}
	return tp, nil
}

// GetPriceHistory get price history
func GetPriceHistory(ctx context.Context, uid int) (priceHistory []TurnipPrice, err error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return getPriceHistory(ctx, client, uid, "Date", firestore.Asc, 0)
}

func getPriceHistory(ctx context.Context, client *firestore.Client, uid int, path string, dir firestore.Direction, limit int) (turnipPriceHistory []TurnipPrice, err error) {
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
		var price TurnipPrice = TurnipPrice{}
		if err = doc.DataTo(&price); err != nil {
			logger.Warn().Err(err).Send()
			return nil, err
		}
		price.Path = fmt.Sprintf("users/%d/games/animal_crossing/price_history/%d", uid, price.Date.Unix())
		turnipPriceHistory = append(turnipPriceHistory, price)
	}
	return turnipPriceHistory, nil
}

// GetWeeklyDTCPriceHistory 获得当前周自周日起的价格。周日是买入价
func GetWeeklyDTCPriceHistory(ctx context.Context, uid int, startDate, endDate time.Time) (turnipPriceHistory []TurnipPrice, err error) {
	client, err := firestore.NewClient(ctx, projectID)
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
		var price TurnipPrice = TurnipPrice{}
		if err = doc.DataTo(&price); err != nil {
			logger.Warn().Err(err).Send()
			return nil, err
		}
		price.Path = fmt.Sprintf("users/%d/games/animal_crossing/price_history/%d", uid, price.Date.Unix())
		turnipPriceHistory = append(turnipPriceHistory, price)
	}
	return turnipPriceHistory, nil
}
