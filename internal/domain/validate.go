package domain

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,63}$`)
var digestPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func ValidateID(field, value string) error {
	if !idPattern.MatchString(value) {
		return NewError(ErrValidation, field, "%s 必须为 2 至 64 位字母、数字、点、下划线或连字符", field)
	}
	return nil
}

func ValidateNewBatch(b *DendroBatch, now time.Time) error {
	if err := ValidateID("batch_id", b.BatchID); err != nil {
		return err
	}
	if err := ValidateID("operator_id", b.OperatorID); err != nil {
		return err
	}
	if strings.TrimSpace(b.SiteCode) == "" {
		return NewError(ErrValidation, "site_code", "站点代码不能为空")
	}
	if strings.TrimSpace(b.Species) == "" {
		return NewError(ErrValidation, "species", "树种不能为空")
	}
	if b.SampledAt.IsZero() || b.SampledAt.After(now) {
		return NewError(ErrValidation, "sampled_at", "采样时间必须有效且不能晚于当前时间")
	}
	if len(b.Cores) == 0 {
		return NewError(ErrValidation, "cores", "至少登记一根样芯")
	}
	seen := map[string]bool{}
	for i := range b.Cores {
		c := &b.Cores[i]
		if err := ValidateID("core_id", c.CoreID); err != nil {
			return err
		}
		if seen[c.CoreID] {
			return NewError(ErrValidation, "cores", "样芯编号 %s 重复", c.CoreID)
		}
		seen[c.CoreID] = true
		if strings.TrimSpace(c.TreeCode) == "" || strings.TrimSpace(c.RadiusCode) == "" {
			return NewError(ErrValidation, "cores", "样芯 %s 的树木代码和半径代码不能为空", c.CoreID)
		}
		c.BatchID = b.BatchID
	}
	sort.Slice(b.Cores, func(i, j int) bool { return b.Cores[i].CoreID < b.Cores[j].CoreID })
	return nil
}

func ValidateImage(c CoreSample) error {
	if strings.TrimSpace(c.PreparationMethod) == "" {
		return NewError(ErrValidation, "preparation_method", "制备方法不能为空")
	}
	if !digestPattern.MatchString(c.ImageDigest) {
		return NewError(ErrValidation, "image_digest", "影像摘要必须是 64 位十六进制 SHA-256")
	}
	if c.MicronsPerPixel <= 0 || c.MicronsPerPixel > 1000 {
		return NewError(ErrValidation, "microns_per_pixel", "像素比例尺必须大于 0 且不超过 1000")
	}
	if c.CapturedAt == nil || c.CapturedAt.IsZero() {
		return NewError(ErrValidation, "captured_at", "采集时间不能为空")
	}
	return nil
}

func ValidateObservation(o RingObservation) error {
	if err := ValidateID("observation_id", o.ObservationID); err != nil {
		return err
	}
	if err := ValidateID("core_id", o.CoreID); err != nil {
		return err
	}
	if o.RingIndex < 1 {
		return NewError(ErrValidation, "ring_index", "年轮序号必须从 1 开始")
	}
	if o.CalendarYear < 1000 || o.CalendarYear > 3000 {
		return NewError(ErrValidation, "calendar_year", "公历年份超出支持范围")
	}
	if o.BoundaryPosition < 0 {
		return NewError(ErrValidation, "boundary_position", "边界位置不能为负数")
	}
	if o.MarkerKind == "" {
		o.MarkerKind = MarkerNone
	}
	if o.MarkerKind != MarkerNone && o.MarkerKind != MarkerMissing && o.MarkerKind != MarkerFalse {
		return NewError(ErrValidation, "marker_kind", "未知年轮标记")
	}
	return nil
}

func ValidateReplacementObservation(o RingObservation) error {
	if err := ValidateObservation(o); err != nil {
		return err
	}
	if o.WidthMicrons <= 0 || o.WidthMicrons > 10000 {
		return NewError(ErrValidation, "width_microns", "替换年轮宽度必须大于 0 且不超过 10000 微米")
	}
	if o.MarkerKind != MarkerNone && strings.TrimSpace(o.MarkerNote) == "" {
		return NewError(ErrValidation, "marker_note", "缺轮或伪轮替换必须提供标记解释")
	}
	return nil
}

func EnsureState(batch *DendroBatch, allowed ...State) error {
	if batch.State == StateSealed {
		return NewError(ErrState, "state", "批次已封存，不允许修改")
	}
	for _, s := range allowed {
		if batch.State == s {
			return nil
		}
	}
	return NewError(ErrState, "state", "状态 %s 不允许执行此操作", batch.State)
}

func HasCore(batch *DendroBatch, id string) bool {
	for _, c := range batch.Cores {
		if c.CoreID == id {
			return true
		}
	}
	return false
}
