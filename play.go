package googleplay

import (
   "bufio"
   "encoding/json"
   "github.com/89z/format"
   "github.com/89z/format/crypto"
   "net/http"
   "net/url"
   "os"
   "path/filepath"
   "strconv"
   "strings"
   "time"
)

const (
   Sleep = 4 * time.Second
   agent = "Android-Finsky (sdk=99,versionCode=99999999)"
   origin = "https://android.clients.google.com"
)

var LogLevel format.LogLevel

type Delivery struct {
   DownloadURL string
   SplitDeliveryData []SplitDeliveryData
}

type Details struct {
   Title string
   UploadDate string
   VersionString string
   VersionCode uint64
   NumDownloads uint64
   Size uint64
   Micros uint64
   CurrencyCode string
}

func (d Details) String() string {
   buf := []byte("Title: ")
   buf = append(buf, d.Title...)
   buf = append(buf, "\nUploadDate: "...)
   buf = append(buf, d.UploadDate...)
   buf = append(buf, "\nVersionString: "...)
   buf = append(buf, d.VersionString...)
   buf = append(buf, "\nVersionCode: "...)
   buf = strconv.AppendUint(buf, d.VersionCode, 10)
   buf = append(buf, "\nNumDownloads: "...)
   buf = append(buf, format.Number.GetUint64(d.NumDownloads)...)
   buf = append(buf, "\nSize: "...)
   buf = append(buf, format.Size.GetUint64(d.Size)...)
   buf = append(buf, "\nOffer: "...)
   buf = strconv.AppendFloat(buf, float64(d.Micros)/1e6, 'f', 2, 64)
   buf = append(buf, ' ')
   buf = append(buf, d.CurrencyCode...)
   return string(buf)
}

type SplitDeliveryData struct {
   ID string
   DownloadURL string
}

type Token struct {
   Token string
}

// Request refresh token.
func NewToken(email, password string) (*Token, error) {
   val := url.Values{
      "Email": {email},
      "Passwd": {password},
      // Instead of the following two, you can use this:
      // sdk_version=20
      // but I couldnt get newer versions to work, so I think this is the
      // better option.
      "client_sig": {""},
      "droidguard_results": {""},
   }.Encode()
   req, err := http.NewRequest(
      "POST", origin + "/auth", strings.NewReader(val),
   )
   if err != nil {
      return nil, err
   }
   req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
   hello, err := crypto.ParseJA3(crypto.AndroidAPI26)
   if err != nil {
      return nil, err
   }
   LogLevel.Dump(req)
   res, err := crypto.Transport(hello).RoundTrip(req)
   if err != nil {
      return nil, err
   }
   defer res.Body.Close()
   buf := bufio.NewScanner(res.Body)
   for buf.Scan() {
      kv := strings.SplitN(buf.Text(), "=", 2)
      if len(kv) == 2 && kv[0] == "Token" {
         var tok Token
         tok.Token = kv[1]
         return &tok, nil
      }
   }
   return nil, notFound{"Token"}
}

func OpenToken(name string) (*Token, error) {
   file, err := os.Open(name)
   if err != nil {
      return nil, err
   }
   defer file.Close()
   tok := new(Token)
   if err := json.NewDecoder(file).Decode(tok); err != nil {
      return nil, err
   }
   return tok, nil
}

// Exchange refresh token for access token.
func (t Token) Auth() (*Auth, error) {
   val := url.Values{
      "Token": {t.Token},
      "service": {"oauth2:https://www.googleapis.com/auth/googleplay"},
   }.Encode()
   req, err := http.NewRequest(
      "POST", origin + "/auth", strings.NewReader(val),
   )
   if err != nil {
      return nil, err
   }
   req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
   LogLevel.Dump(req)
   res, err := new(http.Transport).RoundTrip(req)
   if err != nil {
      return nil, err
   }
   defer res.Body.Close()
   if res.StatusCode != http.StatusOK {
      return nil, errorString(res.Status)
   }
   buf := bufio.NewScanner(res.Body)
   for buf.Scan() {
      kv := strings.SplitN(buf.Text(), "=", 2)
      if len(kv) == 2 && kv[0] == "Auth" {
         var auth Auth
         auth.Auth = kv[1]
         return &auth, nil
      }
   }
   return nil, notFound{"Auth"}
}

func (t Token) Create(name string) error {
   err := os.MkdirAll(filepath.Dir(name), os.ModeDir)
   if err != nil {
      return err
   }
   file, err := os.Create(name)
   if err != nil {
      return err
   }
   defer file.Close()
   enc := json.NewEncoder(file)
   enc.SetIndent("", " ")
   return enc.Encode(t)
}

type errorString string

func (e errorString) Error() string {
   return string(e)
}

type notFound struct {
   input string
}

func (n notFound) Error() string {
   return strconv.Quote(n.input) + " not found"
}
