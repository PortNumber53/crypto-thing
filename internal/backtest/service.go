package backtest

import (
	"context"
	"errors"
	"fmt"
)

var ErrNoCandles = errors.New("no candles found for requested range")

// RunRequest is the application-level request shared by CLI and HTTP adapters.
type RunRequest struct {
	Params     Params
	Save       bool
	StrategyID *int64
}

// CombinationRequest adds result limiting to a normal run request.
type CombinationRequest struct {
	RunRequest
	Top int
}

// Runner is implemented by Service and fakes used by REST contract tests.
type Runner interface {
	Run(context.Context, RunRequest) (Result, error)
	RunCombinations(context.Context, CombinationRequest) ([]Result, error)
}

// Service loads candles, executes the engine, and optionally persists results.
type Service struct {
	dbURL string
}

func NewService(dbURL string) *Service { return &Service{dbURL: dbURL} }

func (s *Service) Run(ctx context.Context, request RunRequest) (Result, error) {
	params := request.Params.Normalized()
	if err := params.Validate(); err != nil {
		return Result{}, err
	}
	candles, err := LoadCandles(ctx, s.dbURL, params.Exchange, params.ProductID, params.Granularity, params.Start, params.End)
	if err != nil {
		return Result{}, fmt.Errorf("load candles: %w", err)
	}
	if len(candles) == 0 {
		return Result{}, ErrNoCandles
	}
	result, err := Run(candles, params)
	if err != nil {
		return Result{}, err
	}
	result.StrategyID = request.StrategyID
	if request.Save {
		if err := SaveResult(ctx, s.dbURL, &result); err != nil {
			return Result{}, fmt.Errorf("save result: %w", err)
		}
	}
	return result, nil
}

func (s *Service) RunCombinations(ctx context.Context, request CombinationRequest) ([]Result, error) {
	params := request.Params.Normalized()
	if err := params.Validate(); err != nil {
		return nil, err
	}
	candles, err := LoadCandles(ctx, s.dbURL, params.Exchange, params.ProductID, params.Granularity, params.Start, params.End)
	if err != nil {
		return nil, fmt.Errorf("load candles: %w", err)
	}
	if len(candles) == 0 {
		return nil, ErrNoCandles
	}
	results, err := RunCombinations(candles, params, request.Top)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].StrategyID = request.StrategyID
		if request.Save {
			if err := SaveResult(ctx, s.dbURL, &results[i]); err != nil {
				return nil, fmt.Errorf("save result %s: %w", results[i].Signals, err)
			}
		}
	}
	return results, nil
}
