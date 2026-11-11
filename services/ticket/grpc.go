package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ticketv1 "nongnooch/platform/gen/go/ticket/v1"
)

// GRPCServer implements ticketv1.TicketServiceServer.
type GRPCServer struct {
	ticketv1.UnimplementedTicketServiceServer
	Svc *Service
}

func (g *GRPCServer) GetTickets(ctx context.Context, req *ticketv1.GetTicketsRequest) (*ticketv1.GetTicketsResponse, error) {
	ts, err := g.Svc.ByOrder(ctx, req.OrderId)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal")
	}
	out := &ticketv1.GetTicketsResponse{Complete: len(ts) > 0} // an order's tickets are issued in one transaction
	for _, t := range ts {
		out.Tickets = append(out.Tickets, &ticketv1.Ticket{TicketId: t.TicketID, Kind: t.Kind, VisitDate: t.VisitDate,
			HolderName: t.HolderName, Status: t.Status, Qr: t.QR})
	}
	return out, nil
}

func (g *GRPCServer) VoidTickets(ctx context.Context, req *ticketv1.VoidTicketsRequest) (*ticketv1.VoidTicketsResponse, error) {
	n, err := g.Svc.Void(ctx, req.OrderId, req.OrderItemIds)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal")
	}
	return &ticketv1.VoidTicketsResponse{Voided: int32(n)}, nil
}
