// Sub-perintah CLI binary server.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/config"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/rls"
)

const usage = `Pemakaian:
  ./server                      jalankan server (mode normal)
  ./server make-admin <email>   jadikan akun ber-email itu admin

Contoh (Docker):
  docker compose exec app ./server make-admin anda@domain.com
`

// runCommand menjalankan sub-perintah lalu mengembalikan exit code.
func runCommand(cfg *config.Config, args []string) int {
	switch args[0] {
	case "make-admin":
		if len(args) != 2 {
			fmt.Fprint(os.Stderr, "Email wajib diisi.\n\n"+usage)
			return 2
		}
		return makeAdmin(cfg, args[1])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Perintah tidak dikenal: %q\n\n%s", args[0], usage)
		return 2
	}
}

func makeAdmin(cfg *config.Config, email string) int {
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal konek database: %v\n", err)
		return 1
	}
	// Pastikan tabel ada (instalasi yang belum pernah menyalakan server).
	if err := database.AutoMigrate(db); err != nil {
		fmt.Fprintf(os.Stderr, "Gagal migrasi database: %v\n", err)
		return 1
	}
	// Driver pgx-rls wajib terdaftar sama seperti saat server jalan.
	rls.Register(db)

	user, already, err := database.PromoteToAdmin(db, email)
	switch {
	case errors.Is(err, database.ErrUserNotFound):
		fmt.Fprintf(os.Stderr, "Akun %q belum terdaftar. Daftar dulu lewat halaman Register, lalu jalankan perintah ini lagi.\n", email)
		return 1
	case err != nil:
		fmt.Fprintf(os.Stderr, "Gagal: %v\n", err)
		return 1
	case already:
		fmt.Printf("%s sudah admin — tidak ada yang diubah.\n", user.Email)
		return 0
	}
	fmt.Printf("✓ %s sekarang admin. Refresh halaman — menu Admin langsung muncul, tanpa login ulang.\n", user.Email)
	return 0
}
