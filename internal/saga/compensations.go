package saga

import (
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

// todo: сделать так чтобы можно было передать параметры
type Compensation struct {
	Activity any
	Params   []any
}

type Compensations []Compensation

func (s *Compensations) AddCompensation(activity Compensation) {
	*s = append(*s, activity)
}

func (s Compensations) Compensate(ctx workflow.Context) {
	logger := workflow.GetLogger(ctx)
	logger.Debug("start compensate")

	selector := workflow.NewSelector(ctx)
	for i := 0; i < len(s); i++ {
		logger.Debug("start compensate:", zap.Int("index", i))
		execution := workflow.ExecuteActivity(ctx, s[i].Activity, s[i].Params)
		selector.AddFuture(execution, func(f workflow.Future) {
			if errCompensation := f.Get(ctx, nil); errCompensation != nil {
				workflow.GetLogger(ctx).Error("Executing compensation failed", "Error", errCompensation)
			}
		})
		logger.Debug("end compensate:", zap.Int("index", i))
	}
	for range s {
		selector.Select(ctx)
	}
	logger.Debug("end compensate")
}
