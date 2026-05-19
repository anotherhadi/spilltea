package util

// CursorMovePage moves cursor forward or backward by one page (perPage items),
// clamped to [0, total-1].
func CursorMovePage(cursor, total, perPage int, forward bool) int {
	step := perPage
	if step < 1 {
		step = 1
	}
	if forward {
		cursor += step
	} else {
		cursor -= step
	}
	if cursor < 0 || total <= 0 {
		return 0
	}
	if cursor >= total {
		return total - 1
	}
	return cursor
}

// CursorGotoBottom returns the last valid cursor index for a list of total items.
func CursorGotoBottom(total int) int {
	if total <= 0 {
		return 0
	}
	return total - 1
}
