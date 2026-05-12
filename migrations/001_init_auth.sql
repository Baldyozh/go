-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Roles table
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User-Role junction table
CREATE TABLE IF NOT EXISTS user_roles (
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);

-- Role-Permission junction table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INTEGER REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Decryption requests log table
CREATE TABLE IF NOT EXISTS decryption_requests (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    log_id VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    granted BOOLEAN DEFAULT TRUE,
    reason TEXT
);

-- Insert default roles
INSERT INTO roles (name, description) VALUES 
('integration_developer', 'Разработчик интеграций - доступ к поиску и фильтрации логов'),
('incident_analyst', 'Аналитик инцидентов - доступ к расшифровке конфиденциальных данных'),
('admin', 'Администратор системы - полное управление доступами')
ON CONFLICT (name) DO NOTHING;

-- Insert default permissions
INSERT INTO permissions (name, description) VALUES
('logs:read', 'Чтение логов'),
('logs:search', 'Поиск логов по request_id'),
('logs:filter', 'Фильтрация логов'),
('logs:decrypt', 'Расшифровка конфиденциальных данных'),
('logs:stats', 'Просмотр статистики логов'),
('users:manage', 'Управление пользователями и ролями')
ON CONFLICT (name) DO NOTHING;

-- Assign permissions to roles
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE (r.name, p.name) IN (
    ('integration_developer', 'logs:read'),
    ('integration_developer', 'logs:search'),
    ('integration_developer', 'logs:filter'),
    ('integration_developer', 'logs:stats'),
    ('incident_analyst', 'logs:read'),
    ('incident_analyst', 'logs:search'),
    ('incident_analyst', 'logs:filter'),
    ('incident_analyst', 'logs:decrypt'),
    ('incident_analyst', 'logs:stats'),
    ('admin', 'logs:read'),
    ('admin', 'logs:search'),
    ('admin', 'logs:filter'),
    ('admin', 'logs:decrypt'),
    ('admin', 'logs:stats'),
    ('admin', 'users:manage')
)
ON CONFLICT DO NOTHING;

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_decryption_requests_user_id ON decryption_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_decryption_requests_log_id ON decryption_requests(log_id);
CREATE INDEX IF NOT EXISTS idx_decryption_requests_requested_at ON decryption_requests(requested_at);
