package email

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"time"
)

// Config holds email service configuration
type Config struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	FromName     string
}

// Service handles sending emails
type Service struct {
	config    *Config
	templates map[string]*template.Template
}

// NewService creates a new email service
func NewService(config *Config) *Service {
	service := &Service{
		config:    config,
		templates: make(map[string]*template.Template),
	}

	// Load built-in templates
	service.loadTemplates()

	return service
}

// loadTemplates loads email templates
func (s *Service) loadTemplates() {
	// Verification email template
	verificationTemplate := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4a5568; color: white; padding: 20px; text-align: center; }
        .content { background: #f7fafc; padding: 30px; }
        .button { display: inline-block; padding: 12px 24px; background: #4299e1; color: white; text-decoration: none; border-radius: 4px; margin: 20px 0; }
        .footer { text-align: center; color: #718096; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Verify Your Email</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>Thank you for signing up! Please verify your email address by clicking the button below:</p>
            <p style="text-align: center;">
                <a href="{{.VerificationURL}}" class="button">Verify Email</a>
            </p>
            <p>Or copy and paste this link into your browser:</p>
            <p><a href="{{.VerificationURL}}">{{.VerificationURL}}</a></p>
            <p>This link will expire in 24 hours.</p>
            <p>If you didn't create an account, please ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; {{.Year}} PocketBase Enterprise. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`

	// Use template.Must to ensure template parsing errors are caught at startup
	s.templates["verification"] = template.Must(template.New("verification").Parse(verificationTemplate))

	// Password reset template
	passwordResetTemplate := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4a5568; color: white; padding: 20px; text-align: center; }
        .content { background: #f7fafc; padding: 30px; }
        .button { display: inline-block; padding: 12px 24px; background: #e53e3e; color: white; text-decoration: none; border-radius: 4px; margin: 20px 0; }
        .footer { text-align: center; color: #718096; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Reset Your Password</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>We received a request to reset your password. Click the button below to create a new password:</p>
            <p style="text-align: center;">
                <a href="{{.ResetURL}}" class="button">Reset Password</a>
            </p>
            <p>Or copy and paste this link into your browser:</p>
            <p><a href="{{.ResetURL}}">{{.ResetURL}}</a></p>
            <p>This link will expire in 1 hour.</p>
            <p>If you didn't request this, please ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; {{.Year}} PocketBase Enterprise. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`

	s.templates["password_reset"] = template.Must(template.New("password_reset").Parse(passwordResetTemplate))
}

// SendVerificationEmail sends an email verification email
func (s *Service) SendVerificationEmail(to, name, verificationToken string, baseURL string) error {
	verificationURL := fmt.Sprintf("%s/api/enterprise/users/verify?token=%s", baseURL, verificationToken)

	data := map[string]interface{}{
		"Name":            name,
		"VerificationURL": verificationURL,
		"Year":            time.Now().Year(),
	}

	var body bytes.Buffer
	if err := s.templates["verification"].Execute(&body, data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.sendEmail(to, "Verify Your Email", body.String())
}

// SendPasswordResetEmail sends a password reset email
func (s *Service) SendPasswordResetEmail(to, name, resetToken string, baseURL string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, resetToken)

	data := map[string]interface{}{
		"Name":     name,
		"ResetURL": resetURL,
		"Year":     time.Now().Year(),
	}

	var body bytes.Buffer
	if err := s.templates["password_reset"].Execute(&body, data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.sendEmail(to, "Reset Your Password", body.String())
}

// sendEmail sends an email using SMTP
func (s *Service) sendEmail(to, subject, body string) error {
	// If SMTP is not configured, log and skip (for development)
	if s.config.SMTPHost == "" {
		fmt.Printf("[Email] Would send email to %s: %s\n", to, subject)
		return nil
	}

	// Set up authentication
	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)

	// Compose message
	from := fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromAddress)
	message := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", from, to, subject, body)

	// Send email
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	return smtp.SendMail(addr, auth, s.config.FromAddress, []string{to}, []byte(message))
}

// GenerateToken generates a random token for verification
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
