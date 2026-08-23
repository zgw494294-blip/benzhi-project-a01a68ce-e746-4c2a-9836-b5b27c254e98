package httpui

import (
	"fmt"
	"time"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/domain"
)

func RunSelfcheck(app *application.Service) error {
	now := time.Now().UTC().Truncate(time.Second)
	cfg := domain.SessionConfig{
		Name: "自检评审", ProductCategory: "果汁", HostUserID: "host",
		ReviewerUserIDs: []string{"r1", "r2"}, ScheduledAt: now, Seed: "selfcheck",
		Scales: []domain.ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10, Order: 1}},
	}
	view, err := app.CreateSession(cfg, "host", "self-create")
	if err != nil {
		return err
	}
	id := view.ID
	inputs := []domain.SampleInput{{ID: "s1", InternalCode: "I-1", DisplayName: "样品甲"}, {ID: "s2", InternalCode: "I-2", DisplayName: "样品乙"}}
	for _, sample := range inputs {
		view, err = app.AddSample(id, sample, "host", view.Version)
		if err != nil {
			return err
		}
	}
	view, err = app.Freeze(id, "host", view.Version)
	if err != nil {
		return err
	}
	view, err = app.Start(id, "host", view.Version)
	if err != nil {
		return err
	}
	for _, reviewer := range []string{"r1", "r2"} {
		tasks, taskErr := app.Tasks(id, reviewer)
		if taskErr != nil {
			return taskErr
		}
		for _, task := range tasks {
			view, taskErr = app.Submit(id, domain.EvaluationInput{AssignmentID: task.ID, Scores: map[string]float64{"taste": 7}, StartedAt: now.Add(-20 * time.Second)}, reviewer, view.Version, "eval-"+reviewer+task.ID)
			if taskErr != nil {
				return taskErr
			}
		}
	}
	view, err = app.Close(id, "host", view.Version)
	if err != nil {
		return err
	}
	for _, finding := range view.Findings {
		view, err = app.Resolve(id, finding.ID, domain.ResolutionAccept, "quality", view.Version, "resolve-"+finding.ID)
		if err != nil {
			return err
		}
	}
	view, err = app.Approve(id, "quality", "quality", view.Version, "approve-quality")
	if err != nil {
		return err
	}
	view, err = app.Approve(id, "auditor", "independent", view.Version, "approve-auditor")
	if err != nil {
		return err
	}
	_, receipt, err := app.Seal(id, "quality", view.Version, "seal")
	if err != nil {
		return err
	}
	sealed, err := app.GetSession(id, "quality")
	if err != nil {
		return err
	}
	if sealed.Status != domain.StatusSealed || receipt.ID == "" {
		return fmt.Errorf("封存结果无效")
	}
	if _, err := app.ExportPackage(receipt.ID); err != nil {
		return fmt.Errorf("封存包导出失败: %w", err)
	}
	return app.Store.Validate()
}
