# Change Streams ve Persistent Search

Bu dokuman, Oba LDAP Server'in real-time degisiklik bildirimi ozelliklerini aciklar.

## Genel Bakis

Change Streams, LDAP dizinindeki entry degisikliklerini (ekleme, guncelleme, silme) real-time olarak izlemenizi saglar. Bu ozellik iki sekilde kullanilabilir:

1. **LDAP Persistent Search** - Standart LDAP client'lar icin (RFC draft-ietf-ldapext-psearch)
2. **Go Internal API** - Oba'yi library olarak kullanan uygulamalar icin

## LDAP Persistent Search

### Nedir?

Persistent Search, LDAP protokolune eklenen bir control'dur. Normal bir search istegi gibi calisir, ancak:
- Baglanti acik kaldigi surece sonuclar akmaya devam eder
- Yeni entry eklendiginde, mevcut entry guncellendiginde veya silindiginde bildirim alirsiniz
- Her degisiklik bir SearchResultEntry olarak gonderilir

### Control Detaylari

| Ozellik | Deger |
|---------|-------|
| OID | 2.16.840.1.113730.3.4.3 |
| Criticality | true veya false |
| Deger | changeTypes, changesOnly, returnECs |

### Parametreler

#### changeTypes (Bitmask)

Hangi degisiklik turlerini izlemek istediginizi belirtir:

| Deger | Anlam |
|-------|-------|
| 1 | Add (yeni entry) |
| 2 | Delete (silinen entry) |
| 4 | Modify (guncellenen entry) |
| 8 | ModDN (DN degisikligi) |
| 15 | Tumu (1+2+4+8) |

#### changesOnly

| Deger | Anlam |
|-------|-------|
| true | Sadece degisiklikleri gonder (mevcut entry'leri gonderme) |
| false | Once mevcut entry'leri gonder, sonra degisiklikleri izle |

#### returnECs (Entry Change Notification)

| Deger | Anlam |
|-------|-------|
| true | Her sonucla birlikte degisiklik bilgisi gonder |
| false | Sadece entry'yi gonder |

### Kullanim Ornekleri

#### ldapsearch ile (OpenLDAP)

```bash
# Tum degisiklikleri izle (mevcut entry'ler dahil)
ldapsearch -H ldap://localhost:389 \
  -x -D "cn=admin,dc=example,dc=com" -w secret \
  -b "dc=example,dc=com" \
  -E 'ps:changeTypes=15/changesOnly=0/returnECs=1' \
  "(objectClass=*)"

# Sadece yeni degisiklikleri izle
ldapsearch -H ldap://localhost:389 \
  -x -D "cn=admin,dc=example,dc=com" -w secret \
  -b "ou=users,dc=example,dc=com" \
  -E 'ps:changeTypes=15/changesOnly=1/returnECs=1' \
  "(objectClass=*)"

# Sadece ekleme ve silmeleri izle
ldapsearch -H ldap://localhost:389 \
  -x -D "cn=admin,dc=example,dc=com" -w secret \
  -b "dc=example,dc=com" \
  -E 'ps:changeTypes=3/changesOnly=1/returnECs=1' \
  "(objectClass=*)"

# Sadece kullanici degisikliklerini izle
ldapsearch -H ldap://localhost:389 \
  -x -D "cn=admin,dc=example,dc=com" -w secret \
  -b "ou=users,dc=example,dc=com" \
  -E 'ps:changeTypes=15/changesOnly=1/returnECs=1' \
  "(objectClass=inetOrgPerson)"
```

#### Python ldap3 ile

```python
from ldap3 import Server, Connection, SUBTREE
from ldap3.extend.standard.persistentSearch import PersistentSearch

server = Server('ldap://localhost:389')
conn = Connection(server, 'cn=admin,dc=example,dc=com', 'secret', auto_bind=True)

# Persistent search baslat
ps = PersistentSearch(
    conn,
    search_base='dc=example,dc=com',
    search_filter='(objectClass=*)',
    search_scope=SUBTREE,
    changes_only=True,
    notifications=True,
    streaming=True
)

# Degisiklikleri dinle
for response in ps.listen():
    if response['type'] == 'searchResEntry':
        print(f"Entry: {response['dn']}")
        print(f"Change: {response.get('controls', {})}")
```

#### Java JNDI ile

```java
import javax.naming.directory.*;
import javax.naming.ldap.*;

// Control olustur
byte[] controlValue = createPersistentSearchControlValue(15, true, true);
Control psControl = new BasicControl(
    "2.16.840.1.113730.3.4.3",  // OID
    true,                        // critical
    controlValue
);

// Search yap
LdapContext ctx = new InitialLdapContext(env, null);
ctx.setRequestControls(new Control[]{psControl});

NamingEnumeration<SearchResult> results = ctx.search(
    "dc=example,dc=com",
    "(objectClass=*)",
    searchControls
);

// Sonuclari isle (blocking)
while (results.hasMore()) {
    SearchResult result = results.next();
    System.out.println("DN: " + result.getNameInNamespace());
}
```

### Entry Change Notification Control

returnECs=1 oldugunda, her SearchResultEntry ile birlikte bir control gonderilir:

| Ozellik | Deger |
|---------|-------|
| OID | 2.16.840.1.113730.3.4.7 |
| changeType | 1=add, 2=delete, 4=modify, 8=modDN |
| previousDN | Sadece modDN icin, onceki DN |
| changeNumber | Degisiklik sira numarasi |

## Go Internal API

Oba'yi Go uygulamanizda library olarak kullaniyorsaniz, Change Streams API'sini dogrudan kullanabilirsiniz.

### Temel Kullanim

```go
package main

import (
    "fmt"
    "github.com/oba-ldap/oba/internal/backend"
    "github.com/oba-ldap/oba/internal/storage/stream"
)

func main() {
    // Backend'e erisim (server kurulumundan)
    be := getBackend()

    // Tum degisiklikleri izle
    sub := be.Watch(stream.MatchAll())
    defer be.Unwatch(sub.ID)

    for event := range sub.Channel {
        fmt.Printf("[%s] %s\n", event.Operation, event.DN)
        if event.Entry != nil {
            fmt.Printf("  Attributes: %v\n", event.Entry.Attributes)
        }
    }
}
```

### Filter Kullanimi

```go
// Belirli bir DN'i izle
sub := be.Watch(stream.MatchDN("cn=admin,dc=example,dc=com"))

// Bir subtree'yi izle
sub := be.Watch(stream.MatchSubtree("ou=users,dc=example,dc=com"))

// Ozel filter
sub := be.Watch(stream.WatchFilter{
    BaseDN: "ou=users,dc=example,dc=com",
    Scope:  stream.ScopeOneLevel,  // Sadece dogrudan cocuklar
    Operations: []stream.OperationType{
        stream.OpInsert,
        stream.OpDelete,
    },
})
```

### Resume (Devam Etme)

Baglanti koptuğunda kaldığınız yerden devam edebilirsiniz:

```go
// Son alinan token'i kaydet
var lastToken uint64

sub := be.Watch(stream.MatchAll())
for event := range sub.Channel {
    lastToken = event.Token
    processEvent(event)
}

// Baglanti koptu, yeniden baglan
sub, err := be.WatchWithResume(stream.MatchAll(), lastToken)
if err == stream.ErrTokenTooOld {
    // Token cok eski, bastan basla
    sub = be.Watch(stream.MatchAll())
}
```

### Event Yapisi

```go
type ChangeEvent struct {
    Token     uint64           // Benzersiz sira numarasi
    Operation OperationType    // OpInsert, OpUpdate, OpDelete, OpModifyDN
    DN        string           // Etkilenen entry'nin DN'i
    Entry     *storage.Entry   // Entry verisi (delete icin nil)
    OldDN     string           // Onceki DN (sadece modifyDN icin)
    Timestamp time.Time        // Olay zamani
}
```

### Backpressure

Subscriber buffer'i dolarsa, yeni event'ler drop edilir:

```go
sub := be.Watch(stream.MatchAll())

// Kac event drop edildi?
dropped := sub.DroppedCount()
if dropped > 0 {
    log.Printf("Warning: %d events dropped", dropped)
}

// Drop sayacini sifirla
dropped = sub.ResetDropped()
```

## Performans

### Limitler

| Parametre | Varsayilan | Aciklama |
|-----------|------------|----------|
| Buffer Size | 256 | Subscriber basina event buffer boyutu |
| Replay Buffer | 4096 | Resume icin saklanan event sayisi |

### Oneriler

1. **changesOnly=true kullanin** - Mevcut entry'leri almak istemiyorsaniz
2. **Dar scope secin** - Tum dizin yerine belirli subtree'leri izleyin
3. **Filter kullanin** - Sadece ilgilendiginiz entry'leri izleyin
4. **Event'leri hizli isleyin** - Buffer dolmasini onleyin

## Sorun Giderme

### "persistent search not supported" Hatasi

Persistent Search control critical olarak gonderildi ama server desteklemiyor. Oba'nin dogru surumunu kullandiginizdan emin olun.

### Event'ler Gelmiyor

1. Dogru BaseDN kullandiginizdan emin olun
2. changeTypes degerini kontrol edin
3. changesOnly=false ile test edin (mevcut entry'leri gormeli)

### Baglanti Kopuyor

1. Timeout ayarlarini kontrol edin
2. Network sorunlarini arastirin
3. Resume mekanizmasini kullanin

## Ilgili Belgeler

- [RFC draft-ietf-ldapext-psearch](https://tools.ietf.org/html/draft-ietf-ldapext-psearch-03) - Persistent Search Control
- [RFC 4533](https://tools.ietf.org/html/rfc4533) - LDAP Content Synchronization (syncrepl)
