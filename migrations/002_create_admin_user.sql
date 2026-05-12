-- Create default admin user
-- Password: admin123 (bcrypt hash)
INSERT INTO users (username, email, password_hash) VALUES 
('admin', 'admin@example.com', '$2a$10$tJWqidWxha2Qvfv2ielBmeHfizFhBj2URGjgG9LVOq/32kU.Qod.O')
ON CONFLICT (username) DO NOTHING;

-- Assign admin role to admin user
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id 
FROM users u, roles r 
WHERE u.username = 'admin' AND r.name = 'admin'
ON CONFLICT DO NOTHING;
