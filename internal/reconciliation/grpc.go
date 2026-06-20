package reconciliation

import (
	"context"

	reconciliationpb "switch/api/proto/reconciliation"
)

type GRPCServer struct {
	reconciliationpb.UnimplementedReconciliationServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) AddRecord(ctx context.Context, req *reconciliationpb.AddRecordRequest) (*reconciliationpb.AddRecordResponse, error) {
	r := req.GetRecord()
	s.svc.AddRecord(Record{
		PaymentID:   r.GetPaymentId(),
		SourceBIC:   r.GetSourceBic(),
		DestBIC:     r.GetDestBic(),
		Amount:      r.GetAmount(),
		Currency:    r.GetCurrency(),
		Status:      r.GetStatus(),
		Matched:     r.GetMatched(),
		Discrepancy: r.GetDiscrepancy(),
	})
	return &reconciliationpb.AddRecordResponse{}, nil
}

func (s *GRPCServer) Match(ctx context.Context, req *reconciliationpb.MatchRequest) (*reconciliationpb.MatchResponse, error) {
	err := s.svc.Match(req.GetPaymentId(), req.GetExpectedStatus())
	if err != nil {
		return &reconciliationpb.MatchResponse{Ok: false, Error: err.Error()}, nil
	}
	return &reconciliationpb.MatchResponse{Ok: true}, nil
}

func (s *GRPCServer) Report(ctx context.Context, _ *reconciliationpb.ReportRequest) (*reconciliationpb.ReportResponse, error) {
	records := s.svc.Report()
	pbRecords := make([]*reconciliationpb.Record, len(records))
	for i, r := range records {
		pbRecords[i] = &reconciliationpb.Record{
			PaymentId:   r.PaymentID,
			SourceBic:   r.SourceBIC,
			DestBic:     r.DestBIC,
			Amount:      r.Amount,
			Currency:    r.Currency,
			Status:      r.Status,
			Matched:     r.Matched,
			Discrepancy: r.Discrepancy,
		}
	}
	return &reconciliationpb.ReportResponse{Records: pbRecords}, nil
}

func (s *GRPCServer) ImportExternal(ctx context.Context, req *reconciliationpb.ImportExternalRequest) (*reconciliationpb.ImportExternalResponse, error) {
	_ = s.svc.ImportExternal(req.GetSource())
	return &reconciliationpb.ImportExternalResponse{}, nil
}
