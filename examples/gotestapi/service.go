package main

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	gotestapiv1 "gotestapi/gen/proto/gotestapi/v1"
	gotestapiv1alpha "gotestapi/gen/proto/gotestapi/v1alpha"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	realUserName    = "You"
	cleanerInterval = time.Second * 10
	deleteAfter     = time.Minute * 20
)

type veggieShopService struct {
	gotestapiv1.UnimplementedVeggieShopServiceServer

	// sync.Map to avoid blocking the whole service during cleanup & other operations.
	shops sync.Map
}

func (vs *veggieShopService) CreateShop(context.Context, *gotestapiv1.CreateShopRequest) (*gotestapiv1.CreateShopResponse, error) {
	id := randomID()
	vs.shops.Store(id, newVeggieShop(id))
	slog.Debug("created new shop", "id", id)
	return &gotestapiv1.CreateShopResponse{ShopId: id}, nil
}

func (vs *veggieShopService) GetShopStats(ctx context.Context, req *gotestapiv1.GetStatsRequest) (*gotestapiv1.GetStatsResponse, error) {
	shop, err := vs.getShop(req.ShopId)
	if err != nil {
		return nil, err
	}

	return &gotestapiv1.GetStatsResponse{PurchasedVeggies: statsToPB(shop.readStats())}, nil
}

func (vs *veggieShopService) BuildPurchase(stream gotestapiv1.VeggieShopService_BuildPurchaseServer) error {
	purchases := make(map[gotestapiv1alpha.Veggie]uint32)

	var shop *veggieShop

	for {
		// Aggregate shopping purchases until the client stream ends.
		// Note that this requires the client to operate using a method which supports client stream closure.
		req, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return status.Errorf(codes.Internal, "receiving purchase request: %s", err)
		}

		shopID := req.ShopId
		if shop == nil {
			shop, err = vs.getShop(shopID)
			if err != nil {
				return err
			}
		} else if shop.id != shopID {
			return status.Errorf(codes.FailedPrecondition, "shop ID changed")
		}

		var veg gotestapiv1alpha.Veggie
		var inc int64

		switch action := req.Action.(type) {
		case *gotestapiv1.BuildPurchaseRequest_Add:
			veg = action.Add
			inc = 1
			shop.emitEvent(veggieShopEvent{User: realUserName, Added: veg})
		case *gotestapiv1.BuildPurchaseRequest_Remove:
			veg = action.Remove
			inc = -1
			shop.emitEvent(veggieShopEvent{User: realUserName, Removed: veg})
		}

		purchases[veg] = max(uint32(int64(purchases[veg])+inc), 0)
	}

	// Tally up the results and update the shop's stats.
	shop.performPurchase(realUserName, purchases)
	return stream.SendAndClose(&gotestapiv1.BuildPurchaseResponse{PurchasedVeggies: statsToPB(purchases)})
}

func (vs *veggieShopService) MonitorShop(req *gotestapiv1.MonitorShopRequest, stream gotestapiv1.VeggieShopService_MonitorShopServer) error {
	shop, err := vs.getShop(req.ShopId)
	if err != nil {
		return err
	}

	ch := shop.addEventChan()
	defer shop.removeEventChan(ch)

	for {
		var event veggieShopEvent

		select {
		case event = <-ch:
		case <-stream.Context().Done():
			return nil
		}

		resp := &gotestapiv1.MonitorShopResponse{User: event.User}
		if event.Added != gotestapiv1alpha.Veggie_UNKNOWN {
			resp.Action = &gotestapiv1.MonitorShopResponse_Add{Add: event.Added}
		} else if event.Removed != gotestapiv1alpha.Veggie_UNKNOWN {
			resp.Action = &gotestapiv1.MonitorShopResponse_Remove{Remove: event.Removed}
		} else {
			resp.Action = &gotestapiv1.MonitorShopResponse_Purchase_{Purchase: &gotestapiv1.MonitorShopResponse_Purchase{Veggies: statsToPB(event.Purchased)}}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (vs *veggieShopService) getShop(shopID string) (*veggieShop, error) {
	shop, ok := vs.shops.Load(shopID)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "shop not found")
	}

	return shop.(*veggieShop), nil
}

func (vs *veggieShopService) cleaner() {
	for {
		time.Sleep(cleanerInterval)
		vs.shops.Range(func(key, value any) bool {
			shop := value.(*veggieShop)
			if time.Since(shop.createdAt) > deleteAfter {
				shop.close()
				vs.shops.Delete(key)
				slog.Debug("deleted old shop", "id", shop.id, "created_at", shop.createdAt)
			}
			return true
		})
	}
}

func statsToPB(stats map[gotestapiv1alpha.Veggie]uint32) []*gotestapiv1.QuantifiedVeggie {
	veggies := make([]*gotestapiv1.QuantifiedVeggie, 0, len(stats))
	for veg, count := range stats {
		veggies = append(veggies, &gotestapiv1.QuantifiedVeggie{Veggie: veg, Quantity: count})
	}

	return veggies
}
