// Package svcutil menampung helper lintas-service: implementasi tunggal
// untuk pola yang dipakai banyak domain (error 503, audit best-effort,
// URL publik, Argon2, password acak, normalisasi teks/nomor, pola validasi,
// konstanta batas field). Leaf: tidak boleh mengimpor subpackage service
// mana pun. Helper baru yang dibutuhkan ≥2 domain WAJIB tinggal di sini,
// bukan sebagai fungsi tak-terekspor di package domain.
package svcutil
