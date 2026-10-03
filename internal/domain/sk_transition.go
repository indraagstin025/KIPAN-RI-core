package domain

// sk_transition.go menyajikan state machine persetujuan SK sebagai DATA (bukan
// if/switch tersebar), sehingga aturan transisi teruji terpusat (golden test)
// dan dipakai oleh service.

// SKApprovalTransition merinci satu transisi SK yang sah. Level kosong berarti
// aturan berlaku untuk semua tingkat (mis. TERUSKAN/SAHKAN/TOLAK).
type SKApprovalTransition struct {
	Action SKApprovalAction
	Level  TingkatWilayah
	From   SKApprovalStatus
	To     SKApprovalStatus
}

// SKApprovalTransitionRules adalah daftar transisi SK yang sah.
var SKApprovalTransitionRules = []SKApprovalTransition{
	// AJUKAN bergantung tingkat SK.
	{SKActionAjukan, LevelKabupaten, SKApprovalStatusDraft, SKApprovalStatusMenungguProvinsi},
	{SKActionAjukan, LevelProvinsi, SKApprovalStatusDraft, SKApprovalStatusMenungguNasional},
	{SKActionAjukan, LevelNasional, SKApprovalStatusDraft, SKApprovalStatusDisetujui},
	// Rantai lanjutan (berlaku umum).
	{SKActionTeruskan, "", SKApprovalStatusMenungguProvinsi, SKApprovalStatusMenungguNasional},
	{SKActionSahkan, "", SKApprovalStatusMenungguNasional, SKApprovalStatusDisetujui},
	{SKActionTolak, "", SKApprovalStatusMenungguNasional, SKApprovalStatusDitolak},
}

// ResolveSKApprovalTransition mengembalikan (from, to) yang sah untuk aksi dan
// tingkat SK tertentu. ok=false bila aksi/tingkat tidak memiliki aturan.
func ResolveSKApprovalTransition(action SKApprovalAction, level TingkatWilayah) (from, to SKApprovalStatus, ok bool) {
	for _, r := range SKApprovalTransitionRules {
		if r.Action != action {
			continue
		}
		if r.Level != "" && r.Level != level {
			continue
		}
		return r.From, r.To, true
	}
	return "", "", false
}
