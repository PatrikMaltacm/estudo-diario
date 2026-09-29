package repository

import (
	"database/sql"

	"estudo-diario/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// InsertNewUser persiste um novo usuário com a senha já em hash bcrypt.
func InsertNewUser(user *model.UserRequestDTO, db *sql.DB) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
	if err != nil {
		return err
	}

	query := "INSERT INTO users (id, name, points, email, password) VALUES ($1, $2, $3, $4, $5)"
	_, errConsult := db.Exec(query, user.Id, user.Name, user.Points, user.Email, passwordHash)
	return errConsult
}

// InsertPointsToUser acumula pontos ao total atual do usuário.
func InsertPointsToUser(userID string, points int, db *sql.DB) error {
	selectQuery := "SELECT points FROM users WHERE id = $1"

	var userCurrentPoints int
	err := db.QueryRow(selectQuery, userID).Scan(&userCurrentPoints)
	if err != nil {
		return err
	}

	query := "UPDATE users set points = $1 WHERE id = $2"
	_, errConsult := db.Exec(query, userCurrentPoints+points, userID)
	return errConsult
}

// UserExistsByEmail verifica se já existe um usuário com o e-mail informado.
func UserExistsByEmail(email string, db *sql.DB) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)"

	var exists bool

	err := db.QueryRow(query, email).Scan(&exists)
	return exists, err
}

// GetCredentialsByEmail busca o id e o hash de senha do usuário pelo e-mail.
func GetCredentialsByEmail(email string, db *sql.DB) (string, []byte, error) {
	var id string
	var hash []byte

	err := db.QueryRow(
		"SELECT id, password FROM users WHERE email = $1",
		email,
	).Scan(&id, &hash)

	return id, hash, err
}
