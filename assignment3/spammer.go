package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// assignment 1
// TODO
// 0)  each 15 min check folder and if new scan was created run ocr and so on...
//    ||
//      simple web with one button load pdf
// 1) OCR pdf and extract Barcode
// 2) Get Email by Barcode from Excel(dao, db) Mysql  - design patterns Strategy
// 3) Send email with attached pdf

const (
	MaxLineLength      = 76                             // MaxLineLength is the maximum line length per RFC 2045
	defaultContentType = "text/plain; charset=us-ascii" // defaultContentType is the default Content-Type according to RFC 2045, section 5.2
)


var (
	host       = "smtp.office365.com"
	username   = ""
	password   = ""
	portNumber = "587"
	// ErrMissingBoundary is returned when there is no boundary given for a multipart entity
	ErrMissingBoundary = errors.New("No boundary found for multipart entity")
	// ErrMissingContentType is returned when there is no "Content-Type" header for a MIME entity
	ErrMissingContentType = errors.New("No Content-Type found for MIME entity")
)
//	username   = "d.kadyrov@astanait.edu.kz"
//	password   = "Harrykane10"


var uploadFormTmpl = []byte ( `
<html> 
	<body>
		<div>
			<form action="/upload" method="post" enctype="multipart/form-data">
				Image: <input type="file" name="my_file" multiple="multiple">
				<input type="submit" value="Upload">
			</form>
		</div>
		<br/>
		{{range .Items}}
		<div>
			<a href="/docs/{{.Path}}.pdf">{{.Path}}.pdf</a>
			<br/>
		</div>
		{{end}}
	</body>
</html>
`)


func main() {
	getEmail:= &Barcode{}
	email := getEmail.GetEmailByBarcode("201506")
	fmt.Println(email)
	fmt.Println(SendEmailWithPDF(email))
	http.HandleFunc("/", List)
	http.HandleFunc("/", mainPage)
	http.HandleFunc("/upload", Upload)
	http.HandleFunc("/raw_body", uploadRawBody)
	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
	staticHandler := http.StripPrefix(
		"/docs/",
		http.FileServer(http.Dir("./docs")),
		)
	http.Handle("/docs", staticHandler)
}

func mainPage(w http.ResponseWriter, r *http.Request) {
	w.Write(uploadFormTmpl)
}

func uploadPage(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(5 * 1024 * 1025)
	file, handler, err := r.FormFile("my_file")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Fprintf(w, "handler.Filename %v\n", handler.Filename)
	fmt.Fprintf(w, "handler.Header %#v\n", handler.Header)

}

type Params struct {
	ID   int
	User string
}


type Document struct {
	code string
	Path string
}

var (
	items =[]*Document{}
)


func List(w http.ResponseWriter, r *http.Request) {
	tmpl :=template.Must(template.New(`list`).Parse(string((uploadFormTmpl))))
	err := tmpl.Execute(w,
		struct {
			Items []*Document
		}{
		items,
		})
	if err!=nil {
		log.Println("Can not execute template", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

var (
	letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func RandStringRunes(n int) string{
	b :=make([]rune, n)
	for i := range b{
		b[i] = letterRunes[int(len(letterRunes))]
	}
	return string(b)
}


func SaveFile(in io.Reader) (string, error) {
	tmpName := RandStringRunes(32)
	tmpFile := "/docs" +tmpName + ".pdf"
	newFile, err :=os.Create(tmpFile)
	if err !=nil{
		return "", err
	}
	hasher :=md5.New()
	_, err = io.Copy(newFile, io.TeeReader(in, hasher))
	newFile.Sync()
	newFile.Close()
	md5Sum := hex.EncodeToString(hasher.Sum(nil))
	if err !=nil{
		return "", err
	}
	realFile := "./docs"+md5Sum +".pdf"
	err = os.Rename(tmpFile, realFile)
	return md5Sum, nil
}

//post method: http://localhost:8080/raw_body

func uploadRawBody(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	p := &Params{}
	err = json.Unmarshal(body, p)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, "content-type %#v\n",
		r.Header.Get("Content-Type"))
	fmt.Fprintf(w, "params %#v\n", p)
}


type Data interface{
	GetEmailByBarcode(barcode string)
	FindEmailByBarcode()
}

type Barcode struct {
	Data Data
}

type Spravki struct{
	barcode string
	email string

}

func Upload(w http.ResponseWriter, r *http.Request){
	err := r.ParseMultipartForm(100000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	uploadData := r.MultipartForm
	files :=uploadData.File["my_file"]

	for i, _:=range files{
		file, err:=files[i].Open()
		defer file.Close()
		if err != nil{
			fmt.Fprintln(w,err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	md5Sum, err := SaveFile(file)
	fileName := "scan1"
	barcode := GetBarcodeFromTesseract(fileName)
	getEmail:= &Barcode{}
	email := getEmail.GetEmailByBarcode(barcode)
	if err != nil {
		log.Println("cant save file", err)
		http.Error(w,"Internal error", http.StatusInternalServerError)
		return
	}
    SendEmailWithPDF(email)
    items = append(items, &Document{
	Path: md5Sum,
  })

	}
	http.Redirect(w, r, "/", 302)
  }

func GetBarcodeFromTesseract(fileName string) string{
	bodyBuf :=&bytes.Buffer{}
	bodyWriter :=multipart.NewWriter(bodyBuf)

	fileWriter, err := bodyWriter.CreateFormFile("the_file", fileName)
	if err != nil{
		fmt.Println("Error writing to buffer")
		return ""
	}

	fh, err:=os.Open(fileName)
	if err!=nil{
		fmt.Println("error opening file")
		return ""
	}
	defer fh.Close()

	_,err = io.Copy(fileWriter, fh)
	if err != nil {
		return ""
	}

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post("http://localhost:8080/api/upload/pdf", contentType, bodyBuf)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil{
		return ""
	}
	text := string(respBody)

	//CHECK!!!!!!!!!!!!!!!!!!

	return text
}




func (b *Barcode) GetEmailByBarcode(barcode string) string{
	var email string
	f, err := excelize.OpenFile("db.xlsx")
	if err != nil {
		println(err.Error())
		return "cannot open file"
	}
	for counter := 2 ; counter <= 11;counter++ {
		code, err := f.GetCellValue("Лист1", "A"+strconv.Itoa(counter))
		if err != nil {
			println(err.Error())
			return "cannot get code from file"
		}
		if code == barcode {
			email, err = f.GetCellValue("Лист1", "B"+strconv.Itoa(counter))
			if err != nil {
				println(err.Error())
				return "cannot get email from file"
			}
			break
		}
	}
	return email
}


//THE FOLLOWING FUNCTION IS IN CASE THERE WILL BE A DATABASE

func (b *Barcode) FindEmailByBarcode(){
	connStr := "user=postgres dbname=spravki password=sysdba host=127.0.0.1"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Print(err)
	}

	var checkDatabase string
	db.QueryRow("SELECT to_regclass('public.student')").Scan(&checkDatabase)
	if err != nil {
		fmt.Print(err)
	}
	if checkDatabase == ""{
		fmt.Println("Database Created")
		createSQL := "CREATE TABLE student (barcode VARCHAR(255),email VARCHAR(255));"
		db.Query(createSQL)
	}

	statement :="INSERT INTO student(barcode, email) VALUES($1, $2)"
	stmt , err := db.Prepare(statement)
	if err != nil {
		fmt.Print(err)
	}
	defer stmt.Close()
	sSpravka := Spravki{}
	for i :=0; i <1700; i++ {
		fmt.Print("Barcode: ")
		fmt.Scanf("%s",&sSpravka.barcode)
		fmt.Print("Email: ")
		fmt.Scanf("%s",&sSpravka.email)
		stmt.QueryRow(sSpravka.barcode,sSpravka.email)
	}

	rows, err := db.Query("SELECT email from student WHERE barcode = ?")
	if err != nil {
		fmt.Print(err)
	}
	defer rows.Close()
	for rows.Next(){
		var email string
		var barcode string
		err := rows.Scan(&barcode, &email)
		if err != nil {
			fmt.Print(err)
		}
		fmt.Printf("%s %s\n",barcode,email)
	}
	return
}


func SendEmailWithPDF(email string) string{
	pathToPdf := "scan1.PDF"
	e := NewEmail()
	e.From = username
	e.To = []string{email}
	e.Subject = "Ваша справка готова"
	e.Text = []byte("Test")
	_,err := e.AttachFile(pathToPdf)
	if err != nil {
		println(err.Error())
		return "error"
	}
	addr := host+":"+portNumber
	auth := LoginAuth(username, password)

	err = e.Send(addr, auth)
	if err != nil {
		println(err.Error())
		return "error"
	}
	return "Success!"
}




// ______________________________________SOME MAGICK DON't TOUCH
type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}


func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unkown fromServer")
		}
	}
	return nil, nil
}


type Email struct {
	ReplyTo     []string
	From        string
	To          []string
	Bcc         []string
	Cc          []string
	Subject     string
	Text        []byte // Plaintext message (optional)
	HTML        []byte // Html message (optional)
	Sender      string // override From as SMTP envelope sender (optional)
	Headers     textproto.MIMEHeader
	Attachments []*Attachment
	ReadReceipt []string
}

// part is a copyable representation of a multipart.Part
type part struct {
	header textproto.MIMEHeader
	body   []byte
}

// NewEmail creates an Email, and returns the pointer to it.
func NewEmail() *Email {
	return &Email{Headers: textproto.MIMEHeader{}}
}



// parseMIMEParts will recursively walk a MIME entity and return a []mime.Part containing
// each (flattened) mime.Part found.
// It is important to note that there are no limits to the number of recursions, so be
// careful when parsing unknown MIME structures!
func parseMIMEParts(hs textproto.MIMEHeader, b io.Reader) ([]*part, error) {
	var ps []*part
	// If no content type is given, set it to the default
	if _, ok := hs["Content-Type"]; !ok {
		hs.Set("Content-Type", defaultContentType)
	}
	ct, params, err := mime.ParseMediaType(hs.Get("Content-Type"))
	if err != nil {
		return ps, err
	}
	// If it's a multipart email, recursively parse the parts
	if strings.HasPrefix(ct, "multipart/") {
		if _, ok := params["boundary"]; !ok {
			return ps, ErrMissingBoundary
		}
		mr := multipart.NewReader(b, params["boundary"])
		for {
			var buf bytes.Buffer
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return ps, err
			}
			if _, ok := p.Header["Content-Type"]; !ok {
				p.Header.Set("Content-Type", defaultContentType)
			}
			subct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if err != nil {
				return ps, err
			}
			if strings.HasPrefix(subct, "multipart/") {
				sps, err := parseMIMEParts(p.Header, p)
				if err != nil {
					return ps, err
				}
				ps = append(ps, sps...)
			} else {
				var reader io.Reader
				reader = p
				const cte = "Content-Transfer-Encoding"
				if p.Header.Get(cte) == "base64" {
					reader = base64.NewDecoder(base64.StdEncoding, reader)
				}
				// Otherwise, just append the part to the list
				// Copy the part data into the buffer
				if _, err := io.Copy(&buf, reader); err != nil {
					return ps, err
				}
				ps = append(ps, &part{body: buf.Bytes(), header: p.Header})
			}
		}
	} else {
		// If it is not a multipart email, parse the body content as a single "part"
		if hs.Get("Content-Transfer-Encoding") == "quoted-printable" {
			b = quotedprintable.NewReader(b)

		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, b); err != nil {
			return ps, err
		}
		ps = append(ps, &part{body: buf.Bytes(), header: hs})
	}
	return ps, nil
}

// Attach is used to attach content from an io.Reader to the email.
// Required parameters include an io.Reader, the desired filename for the attachment, and the Content-Type
// The function will return the created Attachment for reference, as well as nil for the error, if successful.
func (e *Email) Attach(r io.Reader, filename string, c string) (a *Attachment, err error) {
	var buffer bytes.Buffer
	if _, err = io.Copy(&buffer, r); err != nil {
		return
	}
	at := &Attachment{
		Filename: filename,
		Header:   textproto.MIMEHeader{},
		Content:  buffer.Bytes(),
	}
	if c != "" {
		at.Header.Set("Content-Type", c)
	} else {
		at.Header.Set("Content-Type", "application/octet-stream")
	}
	at.Header.Set("Content-Disposition", fmt.Sprintf("attachment;\r\n filename=\"%s\"", filename))
	at.Header.Set("Content-ID", fmt.Sprintf("<%s>", filename))
	at.Header.Set("Content-Transfer-Encoding", "base64")
	e.Attachments = append(e.Attachments, at)
	return at, nil
}

// AttachFile is used to attach content to the email.
// It attempts to open the file referenced by filename and, if successful, creates an Attachment.
// This Attachment is then appended to the slice of Email.Attachments.
// The function will then return the Attachment for reference, as well as nil for the error, if successful.
func (e *Email) AttachFile(filename string) (a *Attachment, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	ct := mime.TypeByExtension(filepath.Ext(filename))
	basename := filepath.Base(filename)
	return e.Attach(f, basename, ct)
}

// msgHeaders merges the Email's various fields and custom headers together in a
// standards compliant way to create a MIMEHeader to be used in the resulting
// message. It does not alter e.Headers.
// "e"'s fields To, Cc, From, Subject will be used unless they are present in
// e.Headers. Unless set in e.Headers, "Date" will filled with the current time.
func (e *Email) msgHeaders() (textproto.MIMEHeader, error) {
	res := make(textproto.MIMEHeader, len(e.Headers)+6)
	if e.Headers != nil {
		for _, h := range []string{"Reply-To", "To", "Cc", "From", "Subject", "Date", "Message-Id", "MIME-Version"} {
			if v, ok := e.Headers[h]; ok {
				res[h] = v
			}
		}
	}
	// Set headers if there are values.
	if _, ok := res["Reply-To"]; !ok && len(e.ReplyTo) > 0 {
		res.Set("Reply-To", strings.Join(e.ReplyTo, ", "))
	}
	if _, ok := res["To"]; !ok && len(e.To) > 0 {
		res.Set("To", strings.Join(e.To, ", "))
	}
	if _, ok := res["Cc"]; !ok && len(e.Cc) > 0 {
		res.Set("Cc", strings.Join(e.Cc, ", "))
	}
	if _, ok := res["Subject"]; !ok && e.Subject != "" {
		res.Set("Subject", e.Subject)
	}
	if _, ok := res["Message-Id"]; !ok {
		id, err := generateMessageID()
		if err != nil {
			return nil, err
		}
		res.Set("Message-Id", id)
	}
	// Date and From are required headers.
	if _, ok := res["From"]; !ok {
		res.Set("From", e.From)
	}
	if _, ok := res["Date"]; !ok {
		res.Set("Date", time.Now().Format(time.RFC1123Z))
	}
	if _, ok := res["MIME-Version"]; !ok {
		res.Set("MIME-Version", "1.0")
	}
	for field, vals := range e.Headers {
		if _, ok := res[field]; !ok {
			res[field] = vals
		}
	}
	return res, nil
}

func writeMessage(buff io.Writer, msg []byte, multipart bool, mediaType string, w *multipart.Writer) error {
	if multipart {
		header := textproto.MIMEHeader{
			"Content-Type":              {mediaType + "; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		}
		if _, err := w.CreatePart(header); err != nil {
			return err
		}
	}

	qp := quotedprintable.NewWriter(buff)
	// Write the text
	if _, err := qp.Write(msg); err != nil {
		return err
	}
	return qp.Close()
}

func (e *Email) categorizeAttachments() (htmlRelated, others []*Attachment) {
	for _, a := range e.Attachments {
		if a.HTMLRelated {
			htmlRelated = append(htmlRelated, a)
		} else {
			others = append(others, a)
		}
	}
	return
}

// Bytes converts the Email object to a []byte representation, including all needed MIMEHeaders, boundaries, etc.
func (e *Email) Bytes() ([]byte, error) {
	// TODO: better guess buffer size
	buff := bytes.NewBuffer(make([]byte, 0, 4096))

	headers, err := e.msgHeaders()
	if err != nil {
		return nil, err
	}

	htmlAttachments, otherAttachments := e.categorizeAttachments()
	if len(e.HTML) == 0 && len(htmlAttachments) > 0 {
		return nil, errors.New("there are HTML attachments, but no HTML body")
	}

	var (
		isMixed       = len(otherAttachments) > 0
		isAlternative = len(e.Text) > 0 && len(e.HTML) > 0
	)

	var w *multipart.Writer
	if isMixed || isAlternative {
		w = multipart.NewWriter(buff)
	}
	switch {
	case isMixed:
		headers.Set("Content-Type", "multipart/mixed;\r\n boundary="+w.Boundary())
	case isAlternative:
		headers.Set("Content-Type", "multipart/alternative;\r\n boundary="+w.Boundary())
	case len(e.HTML) > 0:
		headers.Set("Content-Type", "text/html; charset=UTF-8")
		headers.Set("Content-Transfer-Encoding", "quoted-printable")
	default:
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		headers.Set("Content-Transfer-Encoding", "quoted-printable")
	}
	headerToBytes(buff, headers)
	_, err = io.WriteString(buff, "\r\n")
	if err != nil {
		return nil, err
	}

	// Check to see if there is a Text or HTML field
	if len(e.Text) > 0 || len(e.HTML) > 0 {
		var subWriter *multipart.Writer

		if isMixed && isAlternative {
			// Create the multipart alternative part
			subWriter = multipart.NewWriter(buff)
			header := textproto.MIMEHeader{
				"Content-Type": {"multipart/alternative;\r\n boundary=" + subWriter.Boundary()},
			}
			if _, err := w.CreatePart(header); err != nil {
				return nil, err
			}
		} else {
			subWriter = w
		}
		// Create the body sections
		if len(e.Text) > 0 {
			// Write the text
			if err := writeMessage(buff, e.Text, isMixed || isAlternative, "text/plain", subWriter); err != nil {
				return nil, err
			}
		}
		if len(e.HTML) > 0 {
			messageWriter := subWriter
			var relatedWriter *multipart.Writer
			if len(htmlAttachments) > 0 {
				relatedWriter = multipart.NewWriter(buff)
				header := textproto.MIMEHeader{
					"Content-Type": {"multipart/related;\r\n boundary=" + relatedWriter.Boundary()},
				}
				if _, err := subWriter.CreatePart(header); err != nil {
					return nil, err
				}

				messageWriter = relatedWriter
			}
			// Write the HTML
			if err := writeMessage(buff, e.HTML, isMixed || isAlternative, "text/html", messageWriter); err != nil {
				return nil, err
			}
			if len(htmlAttachments) > 0 {
				for _, a := range htmlAttachments {
					ap, err := relatedWriter.CreatePart(a.Header)
					if err != nil {
						return nil, err
					}
					// Write the base64Wrapped content to the part
					base64Wrap(ap, a.Content)
				}

				relatedWriter.Close()
			}
		}
		if isMixed && isAlternative {
			if err := subWriter.Close(); err != nil {
				return nil, err
			}
		}
	}
	// Create attachment part, if necessary
	for _, a := range otherAttachments {
		ap, err := w.CreatePart(a.Header)
		if err != nil {
			return nil, err
		}
		// Write the base64Wrapped content to the part
		base64Wrap(ap, a.Content)
	}
	if isMixed || isAlternative {
		if err := w.Close(); err != nil {
			return nil, err
		}
	}
	return buff.Bytes(), nil
}

// Send an email using the given host and SMTP auth (optional), returns any error thrown by smtp.SendMail
// This function merges the To, Cc, and Bcc fields and calls the smtp.SendMail function using the Email.Bytes() output as the message
func (e *Email) Send(addr string, a smtp.Auth) error {
	// Merge the To, Cc, and Bcc fields
	to := make([]string, 0, len(e.To)+len(e.Cc)+len(e.Bcc))
	to = append(append(append(to, e.To...), e.Cc...), e.Bcc...)
	for i := 0; i < len(to); i++ {
		addr, err := mail.ParseAddress(to[i])
		if err != nil {
			return err
		}
		to[i] = addr.Address
	}
	// Check to make sure there is at least one recipient and one "From" address
	if e.From == "" || len(to) == 0 {
		return errors.New("Must specify at least one From address and one To address")
	}
	sender, err := e.parseSender()
	if err != nil {
		return err
	}
	raw, err := e.Bytes()
	if err != nil {
		return err
	}
	return smtp.SendMail(addr, a, sender, to, raw)
}

// Select and parse an SMTP envelope sender address.  Choose Email.Sender if set, or fallback to Email.From.
func (e *Email) parseSender() (string, error) {
	if e.Sender != "" {
		sender, err := mail.ParseAddress(e.Sender)
		if err != nil {
			return "", err
		}
		return sender.Address, nil
	} else {
		from, err := mail.ParseAddress(e.From)
		if err != nil {
			return "", err
		}
		return from.Address, nil
	}
}


// Attachment is a struct representing an email attachment.
// Based on the mime/multipart.FileHeader struct, Attachment contains the name, MIMEHeader, and content of the attachment in question
type Attachment struct {
	Filename    string
	Header      textproto.MIMEHeader
	Content     []byte
	HTMLRelated bool
}

// base64Wrap encodes the attachment content, and wraps it according to RFC 2045 standards (every 76 chars)
// The output is then written to the specified io.Writer
func base64Wrap(w io.Writer, b []byte) {
	// 57 raw bytes per 76-byte base64 line.
	const maxRaw = 57
	// Buffer for each line, including trailing CRLF.
	buffer := make([]byte, MaxLineLength+len("\r\n"))
	copy(buffer[MaxLineLength:], "\r\n")
	// Process raw chunks until there's no longer enough to fill a line.
	for len(b) >= maxRaw {
		base64.StdEncoding.Encode(buffer, b[:maxRaw])
		w.Write(buffer)
		b = b[maxRaw:]
	}
	// Handle the last chunk of bytes.
	if len(b) > 0 {
		out := buffer[:base64.StdEncoding.EncodedLen(len(b))]
		base64.StdEncoding.Encode(out, b)
		out = append(out, "\r\n"...)
		w.Write(out)
	}
}

// headerToBytes renders "header" to "buff". If there are multiple values for a
// field, multiple "Field: value\r\n" lines will be emitted.
func headerToBytes(buff io.Writer, header textproto.MIMEHeader) {
	for field, vals := range header {
		for _, subval := range vals {
			// bytes.Buffer.Write() never returns an error.
			io.WriteString(buff, field)
			io.WriteString(buff, ": ")
			// Write the encoded header if needed
			switch {
			case field == "Content-Type" || field == "Content-Disposition":
				buff.Write([]byte(subval))
			case field == "From" || field == "To" || field == "Cc" || field == "Bcc":
				participants := strings.Split(subval, ",")
				for i, v := range participants {
					addr, err := mail.ParseAddress(v)
					if err != nil {
						continue
					}
					if addr.Name != "" {
						participants[i] = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", addr.Name), addr.Address)
					}
				}
				buff.Write([]byte(strings.Join(participants, ", ")))
			default:
				buff.Write([]byte(mime.QEncoding.Encode("UTF-8", subval)))
			}
			io.WriteString(buff, "\r\n")
		}
	}
}

var maxBigInt = big.NewInt(math.MaxInt64)

// generateMessageID generates and returns a string suitable for an RFC 2822
// compliant Message-ID, e.g.:
// <1444789264909237300.3464.1819418242800517193@DESKTOP01>
//
// The following parameters are used to generate a Message-ID:
// - The nanoseconds since Epoch
// - The calling PID
// - A cryptographically random int64
// - The sending hostname
func generateMessageID() (string, error) {
	t := time.Now().UnixNano()
	pid := os.Getpid()
	rint, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return "", err
	}
	h, err := os.Hostname()
	// If we can't get the hostname, we'll use localhost
	if err != nil {
		h = "localhost.localdomain"
	}
	msgid := fmt.Sprintf("<%d.%d.%d@%s>", t, pid, rint, h)
	return msgid, nil
}









