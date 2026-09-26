---
trigger: always_on
---

# GOLANG ENGINEERING RULES — CORE SIM-KIPAN

## 1. Scope & System Boundary

Backend ini adalah **Core SIM-KIPAN**, bukan CMS.

Go Fiber hanya menangani:

* autentikasi dan otorisasi;
* pendaftaran calon anggota;
* verifikasi dan approval anggota;
* data anggota;
* NIA dan KTA;
* Surat Keputusan dan kepengurusan;
* audit trail;
* storage orchestration;
* dashboard agregat;
* public KTA verification;
* integrasi bridge yang memang dibutuhkan Laravel.

Jangan memindahkan fitur CMS seperti berita, galeri, agenda, atau profil organisasi ke Core tanpa keputusan arsitektur eksplisit.

## 2. Follow the Repository Architecture

Gunakan struktur:

`Handler → Service → Repository`

dengan:

* `domain` untuk entity, enum, interface, dan domain error;
* `handler` untuk HTTP transport;
* `service` untuk business logic dan authorization;
* `repository` untuk persistence;
* `middleware` untuk cross-cutting security;
* `pkg` untuk utility reusable.

Jangan mencampurkan tanggung jawab antar-layer.

## 3. Enforce Dependency Direction

Dependency harus bergerak ke dalam:

`Handler → Service → Repository/Infrastructure`

Domain tidak boleh bergantung pada Fiber, database driver, HTTP request, Redis, S3 SDK, atau framework tertentu.

Jangan membuat repository memanggil service.

Jangan menaruh business rule di handler.

## 4. Keep Handlers Thin

Handler hanya bertanggung jawab untuk:

* membaca request;
* parsing DTO;
* validasi input dasar;
* mengambil authenticated context;
* memanggil service;
* mengubah hasil service menjadi HTTP response.

Handler tidak boleh menentukan keputusan bisnis, role, ownership, status transition, atau query database secara langsung.

## 5. Business Logic Belongs in Services

Semua business rule wajib berada di service layer.

Service bertanggung jawab terhadap:

* authorization;
* jurisdiction boundary;
* state transition;
* transaction orchestration;
* validasi business rule;
* audit orchestration;
* external service orchestration.

Jangan mengandalkan frontend untuk menegakkan business rule.

## 6. Never Trust Client Authorization Data

Jangan pernah menentukan authorization berdasarkan:

* `role` dari URL;
* `role` dari request body;
* `user_id` dari request body;
* `provinsi_id` dari client;
* `kabupaten_id` dari client;
* flag seperti `isAdmin`, `isSuperAdmin`, atau `approved`.

Role dan jurisdiction harus berasal dari authenticated server-side identity.

Client hanya meminta aksi; server menentukan apakah aksi tersebut diperbolehkan.

## 7. Enforce Hierarchical RBAC

Role Core adalah:

* `SUPER_ADMIN`
* `ADMIN_NASIONAL`
* `ADMIN_PROVINSI`
* `ADMIN_KABUPATEN`

Boundary wilayah wajib dipaksakan server-side.

`ADMIN_PROVINSI` hanya boleh mengakses provinsinya.

`ADMIN_KABUPATEN` hanya boleh mengakses kabupaten/kotanya.

`ADMIN_NASIONAL` memiliki scope nasional.

`SUPER_ADMIN` memiliki override sesuai matriks kewenangan.

Authorization tidak cukup hanya memeriksa role; object ownership dan jurisdiction juga harus diperiksa.

## 8. Authentication Must Be Server-Side

Semua endpoint internal wajib menggunakan authenticated session.

Gunakan:

* JWT access token dengan lifetime pendek;
* refresh token stateful;
* refresh token rotation;
* token family untuk reuse detection;
* `HttpOnly`, `Secure`, dan `SameSite=Strict` untuk refresh cookie.

Jika refresh token lama yang sudah digunakan kembali terdeteksi, revoke seluruh token family terkait.

Jangan menyimpan secret authentication di source code.

## 9. Password Security

Password wajib menggunakan **Argon2id** dengan konfigurasi yang telah ditetapkan proyek.

Jangan:

* menyimpan plaintext password;
* menggunakan password default produksi;
* menggunakan hash lemah;
* menyediakan endpoint perubahan password user lain tanpa authorization khusus.

Perubahan password sendiri wajib memverifikasi kredensial sesuai policy authentication.

## 10. NIK Must Use Encryption + Blind Index

NIK tidak boleh disimpan plaintext.

Gunakan dua mekanisme terpisah:

`nik_encrypted`
→ AES-256-GCM dengan random nonce.

`nik_hash`
→ HMAC-SHA256 dengan `BLIND_INDEX_KEY`.

`AES_MASTER_KEY` dan `BLIND_INDEX_KEY` wajib berbeda.

Pencarian NIK harus menggunakan blind index, bukan mencari ciphertext.

## 11. Cryptographic Key Management

Secret berikut tidak boleh berada di source code:

* `AES_MASTER_KEY`
* `BLIND_INDEX_KEY`
* `KTA_SIGNING_KEY`
* JWT secrets
* database credentials
* S3 credentials

Pada development/soft-launch gunakan environment secret sesuai blueprint.

Pada production gunakan secret management yang telah direncanakan, yaitu HashiCorp Vault.

Server wajib fail-fast jika secret wajib tidak tersedia atau tidak memenuhi format/ukuran yang diwajibkan.

## 12. Never Expose Sensitive Data by Default

NIK, KTP, dokumen kesehatan, dan dokumen privat tidak boleh dikirim ke client tanpa authorization.

NIK di list/table wajib dimasking.

Dekripsi NIK hanya boleh dilakukan pada context yang memang membutuhkan data tersebut.

Akses terhadap data sensitif wajib dapat diaudit.

Jangan menaruh NIK plaintext, token, secret, password, atau dokumen privat di log.

## 13. Use Private Object Storage

File binary tidak boleh disimpan sebagai Base64 di database.

Gunakan IDCloudHost IS3/S3-compatible object storage.

Database hanya menyimpan object key.

Dokumen sensitif seperti:

* KTP;
* Surat Sehat;
* SK;

wajib berada di private bucket/object namespace.

## 14. Use Presigned Upload/View

Upload dokumen harus menggunakan presigned URL.

Flow standar:

`Request Presign → Upload Direct ke S3 → Submit Object Key`

Backend wajib:

* memvalidasi MIME type;
* memvalidasi ukuran file;
* membatasi kategori file;
* menghasilkan object key yang aman;
* memverifikasi object benar-benar ada sebelum menyimpan reference ke database.

Dokumen privat saat dibuka harus menggunakan temporary presigned GET URL dengan TTL terbatas.

## 15. Never Trust File Metadata Alone

Jangan menganggap file aman hanya karena extension atau `Content-Type` benar.

Validasi file harus mempertimbangkan:

* allowed MIME type;
* ukuran maksimum;
* file signature/magic bytes bila applicable;
* object ownership;
* lokasi/key yang diizinkan;
* kategori file.

Jangan pernah mengeksekusi uploaded file sebagai server-side code.

## 16. Preserve Business State

Data Core yang memiliki nilai bisnis atau legal tidak boleh dihapus secara destruktif hanya untuk mengubah status.

Gunakan status-based lifecycle untuk:

* `pendaftaran`;
* `anggota`;
* `surat_keputusan`;
* `pengurus`.

Master wilayah juga tidak boleh di-hard-delete.

`activity_logs` bersifat append-only.

## 17. Approval Pendaftaran Must Be Transactional

Approval pendaftaran harus menjadi satu business transaction.

Saat pendaftaran disetujui:

1. status pendaftaran menjadi `DISETUJUI`;
2. record pendaftaran tetap ada;
3. anggota baru dibuat;
4. NIA dibuat;
5. KTA diproses;
6. audit history dicatat.

Jangan menghapus row `pendaftaran` setelah approval.

Jika salah satu operasi transactional gagal, sistem harus menjaga atomicity dan konsistensi data.

## 18. NIA Must Be Generated Server-Side

NIA tidak boleh ditentukan oleh client.

Format NIA mengikuti standar:

`KIPAN-[PROV]-[KAB]-[TAHUN]-[NO_URUT]`

NIA wajib:

* generated by backend;
* unique secara nasional;
* konsisten dengan wilayah anggota;
* aman terhadap race condition;
* memiliki database constraint sebagai lapisan terakhir.

Jangan menggunakan sequence logic yang hanya bergantung pada `SELECT MAX(...)`.

## 19. KTA Must Be Server-Side Generated

KTA resmi tidak boleh dibuat atau ditandatangani oleh browser.

Backend bertanggung jawab untuk:

* generate NIA;
* generate signature;
* generate QR;
* render KTA;
* menghasilkan PDF/image resmi;
* menyimpan hasil final ke object storage.

Frontend hanya menampilkan atau mengunduh artefak resmi dari server.

## 20. KTA QR Must Be Cryptographically Verifiable

QR KTA harus mengandung URL verifikasi resmi dengan signature.

Signature menggunakan HMAC dengan `KTA_SIGNING_KEY`.

Backend harus menghitung ulang signature berdasarkan data canonical yang sama dan melakukan constant-time comparison.

Verifikasi publik hanya boleh menampilkan data publik yang memang ditentukan oleh business rule.

QR tidak boleh dianggap valid hanya karena NIA ada di database.

## 21. Audit Every Sensitive Mutation

Semua mutasi sensitif wajib menghasilkan audit trail.

Minimal audit context mencakup:

* `actor_id`;
* `actor_name`;
* `actor_role`;
* timestamp;
* IP;
* user agent;
* request ID;
* action;
* entity/object terkait;
* perubahan old/new value bila applicable.

Data pribadi sensitif wajib dimasking sebelum masuk audit log.

Jangan pernah menggunakan actor statis seperti `"Admin"`.

## 22. Protect Critical Business Transitions

Transition berikut harus diperiksa secara eksplisit:

* pendaftaran;
* verify;
* request revision;
* approve;
* reject;
* anggota status change;
* SK review;
* SK final approval;
* SK deactivate;
* pengurus demisioner.

Jangan mengizinkan arbitrary status assignment seperti:

`PATCH { "status": "..." }`

tanpa pemeriksaan state machine dan authority.

Setiap transition harus memiliki:

* current-state requirement;
* allowed actor;
* jurisdiction requirement;
* side effects;
* audit record.

## 23. Database Access Must Be Safe and Bounded

Gunakan parameterized query.

Jangan melakukan string concatenation untuk query SQL.

Pagination wajib selalu bounded.

Gunakan maximum limit yang telah ditetapkan proyek dan jangan menerima `limit` tidak terbatas dari client.

Query dashboard dan statistik agregat harus menggunakan cache ketika sesuai.

Jangan mengembalikan dataset internal secara massal hanya karena client meminta page size besar.

## 24. Standardize API Contracts and Errors

Semua API Go harus mengikuti contract yang konsisten:

Success:

* `success`;
* `code`;
* `message`;
* `data`;
* `meta` bila paginated.

Error harus memiliki:

* HTTP status yang tepat;
* machine-readable error code;
* human-readable safe message;
* field errors bila validation error;
* `trace_id` / request identifier.

Jangan mengirim stack trace, SQL error, secret, path internal, atau detail infrastructure ke client.

API version menggunakan namespace:

`/api/v1/...`

## 25. Security Middleware and Infrastructure Are Mandatory

Global middleware harus menerapkan minimal:

* request ID;
* panic recovery;
* authentication bila route protected;
* RBAC;
* rate limiting;
* security headers;
* structured logging;
* CORS policy yang eksplisit.

Production wajib menggunakan HTTPS.

Security headers mengikuti baseline proyek, termasuk HSTS, `X-Content-Type-Options`, `X-Frame-Options`, dan `Referrer-Policy`.

Rate limiting wajib terutama diterapkan pada:

* login;
* refresh;
* OTP;
* public tracking;
* public verification;
* upload/presign;
* endpoint sensitif lainnya.

## 26. No Security Regression; Code Must Pass Quality Gates

Setiap perubahan backend wajib mempertahankan:

* Clean Architecture;
* authorization boundary;
* data protection;
* transaction integrity;
* auditability;
* bounded resource usage;
* API compatibility.

Sebelum dianggap siap:

```bash
go test ./... -race -count=1
go vet ./...
gosec ./...
govulncheck ./...
```

Tambahkan unit test untuk domain/service penting dan integration test untuk repository/database/auth flow.

Jangan mengurangi validation, security check, audit logging, atau authorization hanya agar test/build menjadi lebih mudah.

Jangan melakukan perubahan dependency major, schema destructive migration, atau perubahan contract API tanpa keputusan eksplisit dan dokumentasi.
