// Package service adalah lapisan logika bisnis SIM-KIPAN, dipecah per
// bounded context (bukan satu package datar). Tiap subpackage memiliki
// doc.go sendiri yang menjelaskan tanggung jawab + dependensi yang diizinkan.
//
// Peta subpackage:
//
//	svcutil  kernel bersama (error 503, audit best-effort, URL publik,
//	         Argon2, password acak, normalisasi, pola validasi). Leaf.
//	testutil fake repository & actor bersama untuk uji (hanya untuk test).
//	mail     template email murni (hanya stdlib). Leaf.
//	auth     login, sesi, ganti password, reset mandiri.
//	pendaftaran lifecycle pendaftaran + revisi + verifikasi (termasuk NIK reveal).
//	anggota  data kader: CRUD admin, reset password, timeline, export.
//	kepengurusan SK + jabatan + pengurus + PAW/mutasi + kedaluwarsa.
//	wilayah  master + admin wilayah + kodepos.
//	users    manajemen akun admin.
//	notify   OTP, outbox email, worker pengirim, notifikasi in-app.
//	dokumen  storage presign, backup database.
//	kta      dokumen Kartu Tanda Anggota (render + tiket baca).
//	insight  laporan, audit trail, dasbor.
//	platform role, profil organisasi.
//
// Aturan dependensi (dikunci scripts/check-service-imports.ps1):
//   - Subpackage BOLEH mengimpor: config, domain, repository, gateway,
//     pkg/*, svcutil, mail, testutil (test saja).
//   - Impor antar-subpackage service HANYA yang searah & terdaftar:
//     pendaftaran → notify, dokumen, kta, mail; kepengurusan/auth/notify → mail.
//   - DILARANG: impor siklik, impor package service root (kosong),
//     helper lintas-domain di luar svcutil, fake bersama di luar testutil.
//   - Handler tetap thin: hanya HTTP transport + delegasi ke interface service.
//   - Komposisi (wiring konkret) hanya di internal/router (deps.go, wire.go)
//     dan internal/worker (runner.go).
package service
