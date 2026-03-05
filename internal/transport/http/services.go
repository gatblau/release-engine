package http

type PolicyEngine struct{}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{}
}

func (p *PolicyEngine) Evaluate(requestID string) bool {
	// TODO: Go-native evaluator
	return true
}

type IdempotencyService struct{}

func NewIdempotencyService() *IdempotencyService {
	return &IdempotencyService{}
}

func (i *IdempotencyService) Proccess(transactionID string) bool {
	// TODO: Deterministic intake transaction
	return true
}
