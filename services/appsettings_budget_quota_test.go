package services

import (
	"encoding/json"
	"testing"
)

func TestBudgetQuotaSettingUnmarshalJSONSupportsSnakeAndCamelCase(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    BudgetQuotaSetting
	}{
		{
			name: "snake_case",
			payload: `{
				"total": 42,
				"refresh_time": "08:15",
				"refresh_day": 5,
				"refresh_month_day": 20
			}`,
			want: BudgetQuotaSetting{
				Total:           42,
				RefreshTime:     "08:15",
				RefreshDay:      5,
				RefreshMonthDay: 20,
			},
		},
		{
			name: "camel_case_from_frontend",
			payload: `{
				"total": 18,
				"refreshTime": "06:30",
				"refreshWeekday": 2,
				"refreshMonthDay": 11
			}`,
			want: BudgetQuotaSetting{
				Total:           18,
				RefreshTime:     "06:30",
				RefreshDay:      2,
				RefreshMonthDay: 11,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got BudgetQuotaSetting
			if err := json.Unmarshal([]byte(tt.payload), &got); err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}
			if got.Total != tt.want.Total {
				t.Fatalf("Total = %v, want %v", got.Total, tt.want.Total)
			}
			if got.RefreshTime != tt.want.RefreshTime {
				t.Fatalf("RefreshTime = %q, want %q", got.RefreshTime, tt.want.RefreshTime)
			}
			if got.RefreshDay != tt.want.RefreshDay {
				t.Fatalf("RefreshDay = %d, want %d", got.RefreshDay, tt.want.RefreshDay)
			}
			if got.RefreshMonthDay != tt.want.RefreshMonthDay {
				t.Fatalf("RefreshMonthDay = %d, want %d", got.RefreshMonthDay, tt.want.RefreshMonthDay)
			}
		})
	}
}
