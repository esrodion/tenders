package router

import (
	"net/http"
	"tenders/internal/controller"
)

func NewRouter(c *controller.Controller) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/ping", c.Ping)
	mux.HandleFunc("GET /api/tenders", c.GetTenders)
	mux.HandleFunc("POST /api/tenders/new", c.NewTender)
	mux.HandleFunc("GET /api/tenders/my", c.MyTenders)
	mux.HandleFunc("GET /api/tenders/{tenderId}/status", c.TenderStatus)
	mux.HandleFunc("PUT /api/tenders/{tenderId}/status", c.SetTenderStatus)
	mux.HandleFunc("PATCH /api/tenders/{tenderId}/edit", c.EditTender)
	mux.HandleFunc("PUT /api/tenders/{tenderId}/rollback/{version}", c.RollbackTender)
	mux.HandleFunc("POST /api/bids/new", c.NewBid)
	mux.HandleFunc("GET /api/bids/my", c.MyBids)
	mux.HandleFunc("GET /api/bids/{tenderId}/list", c.TenderBids)
	mux.HandleFunc("GET /api/bids/{bidId}/status", c.BidStatus)
	mux.HandleFunc("PUT /api/bids/{bidId}/status", c.SetBidStatus)
	mux.HandleFunc("PATCH /api/bids/{bidId}/edit", c.EditBid)
	mux.HandleFunc("PUT /api/bids/{bidId}/submit_decision", c.BidDecision)
	mux.HandleFunc("PUT /api/bids/{bidId}/feedback", c.BidReview)
	mux.HandleFunc("PUT /api/bids/{bidId}/rollback/{version}", c.BidRollback)
	mux.HandleFunc("GET /api/bids/{tenderId}/reviews", c.GetBidReviews)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("page not found"))
	})

	cors := http.NewServeMux()
	cors.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Accept", "*/*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
		} else {
			mux.ServeHTTP(w, r)
		}
	})

	return cors
}
