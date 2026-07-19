package ocr

import (
	"fmt"
	"strings"
)

// Correction is a user override for one recognized unit's text (task T8;
// spec A6, AC6). It lives alongside the engine result inside Result — same
// store, same envelope — and takes precedence on read (EffectiveUnits) while
// the engine's original text stays untouched in Pages, so an override is
// always revertible. A correction is addressed by page index plus the unit's
// index within that page's Units; replacing a page's engine result (re-OCR,
// SetPage) drops that page's corrections, because the units they pointed at
// are gone (spec A5).
type Correction struct {
	// Page is the zero-based page index the corrected unit is on.
	Page int `json:"page"`

	// Unit is the corrected unit's index within that page's Units.
	Unit int `json:"unit"`

	// Text is the user's replacement text, never empty.
	Text string `json:"text"`
}

// Correct records the user override for a unit, replacing any previous
// override for it (spec A6). It is loud on a target that does not exist —
// no such page in the result, a failed page, a unit index out of range — and
// on empty replacement text (reverting is Revert's job, and deleting
// recognized text is not a correction): those are caller bugs, not user
// conditions.
func (r *Result) Correct(page, unit int, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("correct page %d unit %d: replacement text is empty; use Revert to restore the engine text", page, unit)
	}
	pr := r.page(page)
	if pr == nil {
		return fmt.Errorf("correct page %d unit %d: page has no OCR result", page, unit)
	}
	if pr.Failure != nil {
		return fmt.Errorf("correct page %d unit %d: page failed recognition and has no units", page, unit)
	}
	if unit < 0 || unit >= len(pr.Units) {
		return fmt.Errorf("correct page %d unit %d: page has %d units", page, unit, len(pr.Units))
	}
	for i, c := range r.Corrections {
		if c.Page == page && c.Unit == unit {
			r.Corrections[i].Text = text
			return nil
		}
		if c.Page > page || (c.Page == page && c.Unit > unit) {
			r.Corrections = append(r.Corrections[:i], append([]Correction{{Page: page, Unit: unit, Text: text}}, r.Corrections[i:]...)...)
			return nil
		}
	}
	r.Corrections = append(r.Corrections, Correction{Page: page, Unit: unit, Text: text})
	return nil
}

// Revert removes the user override for a unit, restoring the engine text on
// read. Reverting a unit that has no override is a no-op.
func (r *Result) Revert(page, unit int) {
	for i, c := range r.Corrections {
		if c.Page == page && c.Unit == unit {
			r.Corrections = append(r.Corrections[:i], r.Corrections[i+1:]...)
			return
		}
	}
}

// CorrectionFor returns the user override for a unit, if one exists.
func (r Result) CorrectionFor(page, unit int) (string, bool) {
	for _, c := range r.Corrections {
		if c.Page == page && c.Unit == unit {
			return c.Text, true
		}
	}
	return "", false
}

// EffectiveUnits returns a copy of a page's units with corrections applied —
// the read every consumer (review UI, REQ-2) uses (AC6: overrides take
// precedence). ok is false when the page has no OCR result; a failed page
// returns ok true and nil units (its failure is in Pages). The receiver is
// never modified: originals stay retrievable via Pages and CorrectionFor.
func (r Result) EffectiveUnits(page int) (units []RecognizedUnit, ok bool) {
	pr := r.page(page)
	if pr == nil {
		return nil, false
	}
	if len(pr.Units) == 0 {
		return nil, true
	}
	units = make([]RecognizedUnit, len(pr.Units))
	copy(units, pr.Units)
	for _, c := range r.Corrections {
		if c.Page == page && c.Unit < len(units) {
			units[c.Unit].Text = c.Text
		}
	}
	return units, true
}

// SetPage inserts or replaces a page's engine result, keeping Pages in
// ascending order without duplicates (the Result contract) — the merge step
// OCR runs and explicit re-OCR (spec A5) go through. Replacing a page drops
// that page's corrections: they addressed units that no longer exist.
func (r *Result) SetPage(pr PageResult) {
	kept := r.Corrections[:0]
	for _, c := range r.Corrections {
		if c.Page != pr.Page {
			kept = append(kept, c)
		}
	}
	r.Corrections = kept

	for i, existing := range r.Pages {
		if existing.Page == pr.Page {
			r.Pages[i] = pr
			return
		}
		if existing.Page > pr.Page {
			r.Pages = append(r.Pages[:i], append([]PageResult{pr}, r.Pages[i:]...)...)
			return
		}
	}
	r.Pages = append(r.Pages, pr)
}

// page returns the result entry for a page, or nil if the page was never a
// recognized candidate.
func (r *Result) page(page int) *PageResult {
	for i := range r.Pages {
		if r.Pages[i].Page == page {
			return &r.Pages[i]
		}
	}
	return nil
}
