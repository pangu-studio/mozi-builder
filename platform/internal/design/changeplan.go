package design

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/changeplan"
	"github.com/pangu-studio/mozi-builder/mozi/differ"
)

// ChangePlan diffs the current model document against the previous history
// snapshot and assembles an AI Coding contract. v2 has no manifest, so the
// status is pending whenever the diff is non-empty, no_diff otherwise.
func (s Store) ChangePlan(ctx context.Context, project, module, name string) (*changeplan.Result, error) {
	current, err := s.Get(ctx, project, module, name)
	if err != nil {
		return nil, err
	}
	var curr mozi.ModelIR
	if err = json.Unmarshal(current.Document, &curr); err != nil {
		return nil, err
	}

	var prevDocument json.RawMessage
	var prevVersion string
	err = s.DB.QueryRowContext(ctx, `SELECT version,document FROM design_model_history WHERE project_id=$1 AND module=$2 AND name=$3 AND version<>$4 ORDER BY created_at DESC,version DESC LIMIT 1`, project, module, name, current.Version).Scan(&prevVersion, &prevDocument)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	prev := &mozi.ModelIR{Module: curr.Module, Name: curr.Name, Table: curr.Table}
	if prevDocument != nil {
		prev = &mozi.ModelIR{}
		if err = json.Unmarshal(prevDocument, prev); err != nil {
			return nil, err
		}
	}

	diff := differ.Compare(prev, &curr, prevVersion, current.Version)
	status := changeplan.Pending
	if !diff.HasChanges {
		status = changeplan.NoDiff
	}
	return changeplan.Build(changeplan.Input{
		Model:         &curr,
		Previous:      prev,
		Diff:          diff,
		AffectedFiles: diff.AffectedFiles(),
		Status:        status,
	}), nil
}
