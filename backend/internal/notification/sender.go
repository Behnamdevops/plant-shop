package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"
)

// Message represents an email message to be sent.
type Message struct {
	To      string
	Subject string
	HTML    string
	Plain   string
}

// Sender is the interface for sending emails.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

// SMTPSender implements Sender using SMTP.
type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
	useTLS   bool
}

// NewSMTPSender creates a new SMTP sender.
func NewSMTPSender(host string, port int, username, password, fromEmail, fromName string, useTLS bool) *SMTPSender {
	return &SMTPSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     fromEmail,
		fromName: fromName,
		useTLS:   useTLS,
	}
}

// Send sends an email using SMTP.
func (s *SMTPSender) Send(ctx context.Context, message Message) error {
	// Set a deadline for the entire operation
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create the SMTP client
	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	// Build the address
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	// Connect to SMTP server
	var c *smtp.Client
	var err error
	if s.useTLS {
		tlsConfig := &tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial failed: %w", err)
		}
		c, err = smtp.NewClient(conn, s.host)
		if err != nil {
			return fmt.Errorf("smtp client creation failed: %w", err)
		}
	} else {
		c, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial failed: %w", err)
		}
	}
	defer c.Close()

	// Authenticate if needed
	if auth != nil {
		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	// Set the sender
	if err = c.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail from failed: %w", err)
	}

	// Set the recipient
	if err = c.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp rcpt to failed: %w", err)
	}

	// Send the message body
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}
	defer w.Close()

	// Build the email headers and body
	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("From: %s <%s>\n", s.fromName, s.from))
	buf.WriteString(fmt.Sprintf("To: %s\n", message.To))
	buf.WriteString(fmt.Sprintf("Subject: %s\n", message.Subject))
	buf.WriteString("MIME-Version: 1.0\n")
	buf.WriteString("Content-Type: multipart/alternative; boundary=boundary\n")
	buf.WriteString("\n")
	buf.WriteString("--boundary\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\n\n")
	buf.WriteString(message.Plain)
	buf.WriteString("\n\n")
	buf.WriteString("--boundary\n")
	buf.WriteString("Content-Type: text/html; charset=utf-8\n\n")
	buf.WriteString(message.HTML)
	buf.WriteString("\n\n")
	buf.WriteString("--boundary--\n")

	if _, err = buf.WriteTo(w); err != nil {
		return fmt.Errorf("smtp data write failed: %w", err)
	}

	return nil
}

// NoOpSender is a sender that does nothing (used when emails are disabled).
type NoOpSender struct{}

// Send implements Sender for NoOpSender - it does nothing and returns nil.
func (s *NoOpSender) Send(ctx context.Context, message Message) error {
	return nil
}

// LogSender is a sender that logs messages instead of sending (useful for development).
type LogSender struct{}

// Send implements Sender for LogSender - it logs the message to stdout.
func (s *LogSender) Send(ctx context.Context, message Message) error {
	// Trim HTML tags for plain text display
	plain := stripHTML(message.HTML)
	fmt.Printf("[EMAIL] To: %s, Subject: %s\n", message.To, message.Subject)
	fmt.Printf("[EMAIL] Body (HTML trimmed): %.500s...\n\n", plain)
	return nil
}

// stripHTML removes basic HTML tags for logging purposes.
func stripHTML(html string) string {
	// Simple HTML tag removal - replace tags with spaces
	result := html
	for i := 0; i < len(result); i++ {
		if result[i] == '<' {
			for j := i; j < len(result) && result[j] != '>'; j++ {
				result = result[:i] + result[j+1:]
			}
		}
	}
	// Replace multiple spaces with single space
	result = strings.ReplaceAll(result, "  ", " ")
	return strings.TrimSpace(result)
}

// Template data for email templates.
type EmailTemplateData struct {
	RecipientName string
	OrderID       int64
	CustomerName  string
	CustomerEmail string
	Total         int64
	TotalToman    string
	Message       string
	Status        string
	StatusMessage string
	Items         []OrderItem
	OrderURL      string
}

// buildEmailMessage builds a Message from template data.
func buildEmailMessage(to, subject, htmlTemplate, plainTemplate string, data EmailTemplateData) Message {
	// Render HTML
	htmlBuf := &bytes.Buffer{}
	if tmpl, err := template.New("html").Parse(htmlTemplate); err == nil {
		tmpl.Execute(htmlBuf, data)
	}

	// Render plain text
	plainBuf := &bytes.Buffer{}
	if tmpl, err := template.New("plain").Parse(plainTemplate); err == nil {
		tmpl.Execute(plainBuf, data)
	}

	return Message{
		To:      to,
		Subject: subject,
		HTML:    htmlBuf.String(),
		Plain:   plainBuf.String(),
	}
}

// orderCreatedHTML is the HTML template for order_created emails.
const orderCreatedHTML = `<!DOCTYPE html>
<html dir="rtl" lang="fa">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>سفارش جدید - درخت‌فروشی</title>
</head>
<body style="font-family: 'Tahoma', 'Segoe UI', sans-serif; background-color: #f5f5f5; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <div style="background: linear-gradient(135deg, #2e7d32 0%, #4caf50 100%); padding: 30px; text-align: center;">
            <h1 style="color: #ffffff; margin: 0;"> درخت‌فروشی </h1>
            <p style="color: #e8f5e9; margin: 10px 0 0;"> سفارش جدید ثبت شد </p>
        </div>
        <div style="padding: 30px;">
            <h2 style="color: #333333; margin-top: 0;"> سلام {{.CustomerName}} عزیز، </h2>
            <p style="color: #666666; line-height: 1.6;"> سفارش شما با موفقیت ثبت شد. شماره سفارش: </p>
            <p style="background-color: #e8f5e9; padding: 15px; border-radius: 4px; text-align: center; font-size: 24px; font-weight: bold; color: #2e7d32;"> #{{.OrderID}} </p>
            <div style="background-color: #fafafa; padding: 20px; border-radius: 4px; margin: 20px 0;">
                <table style="width: 100%; border-collapse: collapse;">
                    <thead>
                        <tr style="background-color: #e8f5e9;">
                            <th style="padding: 10px; text-align: right; border-bottom: 2px solid #ddd;"> محصول </th>
                            <th style="padding: 10px; text-align: center; border-bottom: 2px solid #ddd;"> تعداد </th>
                            <th style="padding: 10px; text-align: center; border-bottom: 2px solid #ddd;"> قیمت </th>
                            <th style="padding: 10px; text-align: center; border-bottom: 2px solid #ddd;"> مجموع </th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Items}}
                        <tr>
                            <td style="padding: 10px; text-align: right; border-bottom: 1px solid #eee;"> {{.ProductName}} </td>
                            <td style="padding: 10px; text-align: center; border-bottom: 1px solid #eee;"> {{.Quantity}} </td>
                            <td style="padding: 10px; text-align: center; border-bottom: 1px solid #eee;"> {{.UnitPrice}} تومان </td>
                            <td style="padding: 10px; text-align: center; border-bottom: 1px solid #eee; font-weight: bold;"> {{.Subtotal}} تومان </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            <div style="border-top: 2px solid #ddd; padding-top: 20px;">
                <div style="display: flex; justify-content: space-between; margin-bottom: 10px;">
                    <span style="color: #666666;"> مجموع محصولات </span>
                    <span style="font-weight: bold; color: #333333;"> {{.TotalToman}} </span>
                </div>
                <div style="display: flex; justify-content: space-between; margin-bottom: 10px;">
                    <span style="color: #666666;"> مبلغ کل </span>
                    <span style="font-size: 20px; font-weight: bold; color: #2e7d32;"> {{.TotalToman}} </span>
                </div>
            </div>
            <p style="color: #666666; margin-top: 30px;"> ما زودتر از سفارش شما باخبر می‌شیم. اگر سوالی دارید، لطفاً با ما تماس بگیرید. </p>
            <p style="color: #999999; font-size: 12px; margin-top: 20px;"> این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید. </p>
        </div>
    </div>
</body>
</html>`

// orderCreatedPlain is the plain text template for order_created emails.
const orderCreatedPlain = `سفارش جدید ثبت شد - درخت‌فروشی

سلام {{.CustomerName}} عزیز،

سفارش شما با موفقیت ثبت شد.
شماره سفارش: #{{.OrderID}}

محصولات:
{{range .Items}}- {{.ProductName}} ({{.Quantity}} × {{.UnitPrice}} تومان = {{.Subtotal}} تومان)
{{end}}مجموع سفارش: {{.TotalToman}}

ما زودتر از سفارش شما باخبر می‌شیم.
اگر سوالی دارید، لطفاً با ما تماس بگیرید.

این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید.
`

// paymentSucceededHTML is the HTML template for payment_succeeded emails.
const paymentSucceededHTML = `<!DOCTYPE html>
<html dir="rtl" lang="fa">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>پرداخت موفق - درخت‌فروشی</title>
</head>
<body style="font-family: 'Tahoma', 'Segoe UI', sans-serif; background-color: #f5f5f5; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <div style="background: linear-gradient(135deg, #2e7d32 0%, #4caf50 100%); padding: 30px; text-align: center;">
            <h1 style="color: #ffffff; margin: 0;"> پرداخت موفق </h1>
            <p style="color: #e8f5e9; margin: 10px 0 0;"> سفارش شما پرداخت شد </p>
        </div>
        <div style="padding: 30px;">
            <h2 style="color: #333333; margin-top: 0;"> سلام {{.CustomerName}} عزیز، </h2>
            <p style="color: #666666; line-height: 1.6;"> پرداخت شما برای سفارش با موفقیت انجام شد. </p>
            <p style="background-color: #e8f5e9; padding: 15px; border-radius: 4px; text-align: center; font-size: 24px; font-weight: bold; color: #2e7d32;"> شماره سفارش: #{{.OrderID}} </p>
            <div style="background-color: #fafafa; padding: 20px; border-radius: 4px; margin: 20px 0; text-align: center;">
                <p style="margin: 0; font-size: 18px; color: #666666;"> مبلغ پرداختی </p>
                <p style="margin: 10px 0 0; font-size: 28px; font-weight: bold; color: #2e7d32;"> {{.TotalToman}} </p>
            </div>
            <p style="color: #666666; line-height: 1.6;"> سفارش شما در حال پردازش است. به زودی به شما اطلاع خواهیم داد. </p>
            <p style="color: #999999; font-size: 12px; margin-top: 20px;"> این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید. </p>
        </div>
    </div>
</body>
</html>`

// paymentSucceededPlain is the plain text template for payment_succeeded emails.
const paymentSucceededPlain = `پرداخت موفق - درخت‌فروشی

سلام {{.CustomerName}} عزیز،

پرداخت شما برای سفارش با موفقیت انجام شد.
شماره سفارش: #{{.OrderID}}

مبلغ پرداختی: {{.TotalToman}}

سفارش شما در حال پردازش است.
به زودی به شما اطلاع خواهیم داد.

این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید.
`

// paymentFailedHTML is the HTML template for payment_failed emails.
const paymentFailedHTML = `<!DOCTYPE html>
<html dir="rtl" lang="fa">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>پرداخت ناموفق - درخت‌فروشی</title>
</head>
<body style="font-family: 'Tahoma', 'Segoe UI', sans-serif; background-color: #f5f5f5; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <div style="background: linear-gradient(135deg, #c62828 0%, #f44336 100%); padding: 30px; text-align: center;">
            <h1 style="color: #ffffff; margin: 0;"> پرداخت ناموفق </h1>
            <p style="color: #ffebee; margin: 10px 0 0;"> پرداخت شما انجام نشد </p>
        </div>
        <div style="padding: 30px;">
            <h2 style="color: #333333; margin-top: 0;"> سلام {{.CustomerName}} عزیز، </h2>
            <p style="color: #666666; line-height: 1.6;"> متاسفانه پرداخت شما برای سفارش به پایان نرسید. </p>
            <p style="background-color: #ffebee; padding: 15px; border-radius: 4px; text-align: center; font-size: 24px; font-weight: bold; color: #c62828;"> شماره سفارش: #{{.OrderID}} </p>
            <p style="color: #666666; line-height: 1.6;"> لطفاً مجدداً تلاش کنید یا با پشتیبانی تماس بگیرید. </p>
            <p style="color: #999999; font-size: 12px; margin-top: 20px;"> این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید. </p>
        </div>
    </div>
</body>
</html>`

// paymentFailedPlain is the plain text template for payment_failed emails.
const paymentFailedPlain = `پرداخت ناموفق - درخت‌فروشی

سلام {{.CustomerName}} عزیز،

متاسفانه پرداخت شما برای سفارش به پایان نرسید.
شماره سفارش: #{{.OrderID}}

لطفاً مجدداً تلاش کنید یا با پشتیبانی تماس بگیرید.

این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید.
`

// orderStatusHTML is the HTML template for order status emails.
const orderStatusHTML = `<!DOCTYPE html>
<html dir="rtl" lang="fa">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>وضعیت سفارش - درخت‌فروشی</title>
</head>
<body style="font-family: 'Tahoma', 'Segoe UI', sans-serif; background-color: #f5f5f5; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <div style="background: linear-gradient(135deg, #1976d2 0%, #2196f3 100%); padding: 30px; text-align: center;">
            <h1 style="color: #ffffff; margin: 0;"> وضعیت سفارش </h1>
            <p style="color: #e3f2fd; margin: 10px 0 0;"> سفارش شما به‌روز شد </p>
        </div>
        <div style="padding: 30px;">
            <h2 style="color: #333333; margin-top: 0;"> سلام {{.CustomerName}} عزیز، </h2>
            <p style="color: #666666; line-height: 1.6;"> وضعیت سفارش شما تغییر کرد: </p>
            <p style="background-color: #e3f2fd; padding: 15px; border-radius: 4px; text-align: center; font-size: 24px; font-weight: bold; color: #1976d2;"> شماره سفارش: #{{.OrderID}} </p>
            <p style="background-color: #fff8e1; padding: 20px; border-radius: 4px; margin: 20px 0; text-align: center;">
                <span style="color: #666666;"> وضعیت فعلی: </span>
                <span style="display: block; font-size: 20px; font-weight: bold; color: #fbc02d;"> {{.StatusMessage}} </span>
            </p>
            <p style="color: #666666; line-height: 1.6;"> اگر سوالی دارید، لطفاً با ما تماس بگیرید. </p>
            <p style="color: #999999; font-size: 12px; margin-top: 20px;"> این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید. </p>
        </div>
    </div>
</body>
</html>`

// orderStatusPlain is the plain text template for order status emails.
const orderStatusPlain = `وضعیت سفارش - درخت‌فروشی

سلام {{.CustomerName}} عزیز،

وضعیت سفارش شما به‌روز شد.
شماره سفارش: #{{.OrderID}}

وضعیت فعلی: {{.StatusMessage}}

اگر سوالی دارید، لطفاً با ما تماس بگیرید.

این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید.
`

// orderCancelledHTML is the HTML template for order_cancelled emails.
const orderCancelledHTML = `<!DOCTYPE html>
<html dir="rtl" lang="fa">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>سفارش لغو شد - درخت‌فروشی</title>
</head>
<body style="font-family: 'Tahoma', 'Segoe UI', sans-serif; background-color: #f5f5f5; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <div style="background: linear-gradient(135deg, #6d4c41 0%, #795548 100%); padding: 30px; text-align: center;">
            <h1 style="color: #ffffff; margin: 0;"> سفارش لغو شد </h1>
            <p style="color: #efebe9; margin: 10px 0 0;"> سفارش شما لغو شد </p>
        </div>
        <div style="padding: 30px;">
            <h2 style="color: #333333; margin-top: 0;"> سلام {{.CustomerName}} عزیز، </h2>
            <p style="color: #666666; line-height: 1.6;"> سفارش شما لغو شد. </p>
            <p style="background-color: #efebe9; padding: 15px; border-radius: 4px; text-align: center; font-size: 24px; font-weight: bold; color: #6d4c41;"> شماره سفارش: #{{.OrderID}} </p>
            <div style="background-color: #fff8e1; padding: 20px; border-radius: 4px; margin: 20px 0; text-align: center;">
                <p style="margin: 0; font-size: 18px; color: #666666;"> مبلغ لغو شده </p>
                <p style="margin: 10px 0 0; font-size: 28px; font-weight: bold; color: #fbc02d;"> {{.TotalToman}} </p>
            </div>
            <p style="color: #666666; line-height: 1.6;"> اگر سوالی دارید یا نیاز به کمک دارید، لطفاً با ما تماس بگیرید. </p>
            <p style="color: #999999; font-size: 12px; margin-top: 20px;"> این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید. </p>
        </div>
    </div>
</body>
</html>`

// orderCancelledPlain is the plain text template for order_cancelled emails.
const orderCancelledPlain = `سفارش لغو شد - درخت‌فروشی

سلام {{.CustomerName}} عزیز،

سفارش شما لغو شد.
شماره سفارش: #{{.OrderID}}

مبلغ لغو شده: {{.TotalToman}}

اگر سوالی دارید یا نیاز به کمک دارید، لطفاً با ما تماس بگیرید.

این یک ایمیل خودکار است. لطفاً به آن پاسخ ندهید.
`

// Helper functions for templates
func subTotal(items []OrderItem) int64 {
	var total int64
	for _, item := range items {
		total += item.Subtotal
	}
	return total
}

func formatTomanTemplate(amount int64) string {
	if amount < 0 {
		return "0 تومان"
	}
	toman := amount / 10
	return fmt.Sprintf("%d تومان", toman)
}
