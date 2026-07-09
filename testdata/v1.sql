CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);

CREATE VIEW active_users AS
SELECT id, name, email FROM users WHERE email IS NOT NULL;

CREATE TRIGGER after_user_insert
AFTER INSERT ON users
BEGIN
    UPDATE users SET created_at = datetime('now') WHERE id = NEW.id;
END;