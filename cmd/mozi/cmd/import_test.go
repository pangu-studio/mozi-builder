package cmd

import (
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

func TestModuleMetadataIsDefault(t *testing.T) {
	tests := []struct {
		name string
		mod  *mozi.ModuleIR
		want bool
	}{
		{
			name: "pure defaults from missing _module.yaml",
			mod:  &mozi.ModuleIR{Name: "market", Label: "market", APIPrefix: "market"},
			want: true,
		},
		{
			name: "explicit label differs from name",
			mod:  &mozi.ModuleIR{Name: "market", Label: "卡片市场", APIPrefix: "market"},
			want: false,
		},
		{
			name: "explicit description",
			mod:  &mozi.ModuleIR{Name: "market", Label: "market", Description: "交易市场", APIPrefix: "market"},
			want: false,
		},
		{
			name: "explicit icon",
			mod:  &mozi.ModuleIR{Name: "market", Label: "market", Icon: "shop", APIPrefix: "market"},
			want: false,
		},
		{
			name: "custom api prefix",
			mod:  &mozi.ModuleIR{Name: "market", Label: "market", APIPrefix: "mkt"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moduleMetadataIsDefault(tt.mod); got != tt.want {
				t.Errorf("moduleMetadataIsDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
