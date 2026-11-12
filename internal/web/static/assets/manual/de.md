# Orbithek-Benutzerhandbuch

Dieses Handbuch erklärt die Arbeit mit Orbithek: Seiten lesen und schreiben, Aufgaben verfolgen, zusammenarbeiten und Informationen finden. Es beschreibt die Funktionen so, wie die Oberfläche sie zeigt. Einige Funktionen stehen nur zur Verfügung, wenn die Administration sie eingeschaltet hat; dies ist jeweils vermerkt.

> [!NOTE]
> Man öffnet dieses Handbuch jederzeit über das Menü des Profilbilds oben rechts: **Handbuch**. Die Sprachlinks über dem Text wechseln zwischen Deutsch und Englisch.

## Grundbegriffe

### Räume, Einträge und Typen

Alles, was in Orbithek geschrieben wird, ist ein **Eintrag**: eine Wiki-Seite, eine Aufgabe, eine Entscheidung, ein Sitzungsprotokoll. Woraus ein Eintrag besteht, legt sein **Typ** fest. Ein Typ bestimmt die Felder des Eintrags (zum Beispiel *assignee*, *due* oder *priority*), ob er einen Text hat und ob er einen **Workflow** mit Status wie *open* und *done* durchläuft. Seiten und Aufgaben sind deshalb keine getrennten Systeme, sondern Einträge unterschiedlicher Typen. Ein Typ gehört der ganzen Installation oder einem Raum: Die eigenen Typen eines Raums werden nur für Einträge in diesem Raum verwendet.

Einträge liegen in **Räumen**. Ein Raum enthält die Arbeit eines Teams, eines Projekts oder eines Themas. Jeder Raum hat einen kurzen **Schlüssel** in Grossbuchstaben, etwa `PLAT`; jeder Eintrag darin hat einen Schlüssel aus dem Raumschlüssel und einer Nummer, etwa `PLAT-12`. Seiten bilden einen Baum: Jeder Eintrag hat höchstens eine übergeordnete Seite.

### Rollen und Zugriff

Was man in einem Raum tun darf, hängt von der dortigen **Rolle** ab:

| Rolle | Was sie erlaubt |
| --- | --- |
| Leserecht | Einträge des Raums lesen, kommentieren, persönliche Notizen anlegen. |
| Bearbeitungsrecht | Zusätzlich Einträge erstellen, bearbeiten, verschieben und löschen. |
| Raumadministration | Zusätzlich Rollen im Raum vergeben und seine Einstellungen ändern. |

Die Administration der Installation hat in jedem Raum die Rechte der Raumadministration und verwaltet zudem Typen, Benutzer und die Servereinstellungen. Für einzelne Seiten lässt sich der Zugriff weiter einschränken (siehe *Inhalte schützen*); über die Raumrolle hinaus wird er nie erweitert. Was man nicht lesen darf, erscheint nirgends, weder in Suchergebnissen noch in Listen oder Zählungen.

## Erste Schritte

### Anmelden

Die Anmeldung erfolgt mit Benutzername und Passwort oder per Single Sign-on, sofern die Organisation es eingerichtet hat. Sind Selbstregistrierungen erlaubt, sendet **Konto beantragen** auf der Anmeldeseite einen Antrag, den die Administration bewilligt. Bei vergessenem Passwort fordert man einen Link zum Zurücksetzen per E-Mail an; damit wählt man ein neues Passwort, und die übrigen Sitzungen werden beendet.

### Die obere Leiste

Die Leiste oben auf jeder Seite enthält von links nach rechts:

- **Neun Punkte** (Module): öffnet den Modulwechsler mit Räume, Erstellen, Aufgaben, Heute, Kommentare, Album, Whiteboards, Sternkarte, KI-Zugang, Governance, Konto und, für die Administration, Administration.
- **Startseite**, **Zurück** und **Vorwärts**.
- Das **Logo**, das ebenfalls zur Startseite führt.
- Das **Suchfeld**.
- **+ Neu**: erstellt eine Seite oder einen Eintrag am aktuellen Ort.
- **Heute**: die eigene Tagesnotiz von heute (Alt+Shift+D); siehe *Tagesnotizen*. Auf einem schmalen Bildschirm steht es im Modulwechsler.
- Die **Glocke**: die Benachrichtigungen, mit der Anzahl ungelesener.
- **DE** und **EN**: die Sprache der Oberfläche.
- Den Umschalter für **hellen und dunklen Modus**.
- Den **Lesemodus** (Ctrl+Shift+F, F11): Die Seite füllt den Bildschirm; Esc beendet ihn.
- Das **Profilbild** oder die Initialen: öffnet das Kontomenü mit *Konto und Passwort*, *KI-Zugang*, *Handbuch*, *Administration* (nur für die Administration) und *Abmelden*.

### Die Startseite

Die Startseite begrüsst, zeigt die Anzahl Seiten, Aufgaben und Bilder und bietet einen Schnellzugriff auf die Sternkarte, **Mein Tag** (die Tagesnotiz von heute, mit dem heutigen Datum), die eigenen Aufgaben, Benachrichtigungen, persönlichen Notizen, das Album, die Whiteboards und den KI-Zugang. Darunter stehen die eigenen Räume, als Kacheln oder als Liste; ein Stern markiert einen Raum als Favorit. Gibt es noch keine Räume, findet die Administration dort die ersten Schritte: einen Raum erstellen, mit den Vorlagen Typen hinzufügen (Seite, Aufgabe, Entscheidung, Tagesnotiz) und Personen eine Rolle geben.

## Seiten und Einträge

### Eine Seite oder einen Eintrag erstellen

Man wählt **+ Neu** in der oberen Leiste oder **Neue Seite** in einem Raum. Die Erstellungsansicht fragt nach Raum, Typ und Titel und legt den neuen Eintrag unterhalb der Seite an, von der man kam. Auf einer Seite erstellt **Neue Unterseite** eine Seite darunter. Hat ein Typ einen Starttext, beginnt ein neuer Eintrag damit. Die eigenen Typen eines Raums werden nur angeboten, wenn dieser Raum gewählt ist.

### Einen Eintrag lesen

Ein Eintrag zeigt Titel, Schlüssel, Typ und Status sowie seinen Text. Die Seitenleiste **Zu diesem Eintrag** enthält die Angaben:

- **Eigner und Details**: die Felder des Eintrags, bearbeitbar für alle mit Bearbeitungsrecht.
- **Status**: der aktuelle Status und die möglichen Übergänge (**Status ändern**).
- **Labels**, **Beziehungen** und **Dateien**.
- **Zugriff**: wer lesen und bearbeiten darf.
- **Verlauf**: jede Änderung des Eintrags.

Über dem Text stehen **Exportieren** und **Beobachten** zur Verfügung; die Kommentare folgen unter dem Text.

Der Abschnitt **Zusammenhang** in der Seitenleiste zeigt, was zum Eintrag gehört, ohne in ihm zu stehen: unter **Verwiesen von** die Einträge, die ihn erwähnen, einbetten, verlinken oder mit ihm in Beziehung stehen, jeweils mit dem Satz, in dem die Erwähnung oder der Link steht; unter **Mit gleichen Labels** die Einträge mit seinen Labels, die mit den meisten gemeinsamen zuerst; und unter **In Tagesnotizen** die eigenen Tagesnotizen, die ihn nennen. Es erscheinen nur Einträge, die man lesen darf, und jede Liste zeigt ihre ersten Einträge.

Shift+Klick auf einen Link zu einem anderen Eintrag, oder **Daneben** bei einem Eintrag des Zusammenhangs, öffnet diesen Eintrag in einem Bereich neben dem aktuellen: Man liest ihn dort und bearbeitet seinen Text mit **Bearbeiten**, wenn man darf. Die Adresse behält den Eintrag daneben, so lässt sich das Paar teilen und kommt mit **Zurück** wieder. **Für sich öffnen** zeigt ihn als Seite; Esc oder das × schliesst den Bereich. Auf dem Telefon folgt der Bereich unter der Seite.

### Einen Text bearbeiten

Mit **Bearbeiten** wird der Text bearbeitet, mit **Fertig** abgeschlossen. Änderungen werden laufend gespeichert; die Statuszeile zeigt *Wird gespeichert…* und danach *Alle Änderungen gespeichert.* Die Werkzeugleiste bietet:

- die Blockart: Absatz, Überschrift 1 bis 3, Aufzählungspunkt, nummerierter Punkt, Aufgabe, Zitat, Codeblock sowie Info-, Hinweis-, Warn-, Tipp- und Fehlerbox;
- Fett, Kursiv, Unterstreichen, Durchstreichen, Hervorheben, Code im Text und Ausrichtung;
- **Link** (Ctrl+K): eine Webadresse oder eine Seite bzw. Aufgabe, gesucht nach ihrem Titel;
- **Erwähnen** (@): eine Person oder einen Eintrag;
- **Rückgängig** und **Wiederholen**.

Eingefügtes Markdown wird zu formatiertem Text; eine Tabelle behält die Formatierung ihrer Zellen. Ctrl+Shift+V fügt stattdessen reinen Text ein.

Hat kein Eintrag den Titel, den man nach @ oder als Adresse eines Links tippt, bietet die Liste *«Titel» erstellen als …* an. Der Dialog **Erstellen und erwähnen** fragt nach dem Typ (dem Seitentyp des Raums, wenn man keinen anderen wählt) und dem Raum (dem Raum des bearbeiteten Texts, wenn man keinen anderen wählt); eine Seite im selben Raum kommt unter die bearbeitete Seite und ist wie diese geschützt; aus einer Tagesnotiz kommt sie zuoberst in den Raum. Der neue Eintrag wird dort erwähnt, wo man getippt hat. Typen mit Pflichtfeldern erstellt man stattdessen mit **+ Neu**. Darf man in einem Raum nichts erstellen oder ist der Speicher fast voll, sagt es der Dialog, und es wird nichts erstellt.

### Inhalte einfügen

Das Menü **Einfügen…** fügt Blöcke nach dem Cursor ein:

- **Tabelle**, **Trennlinie**, **Inhaltsverzeichnis**, **Bildverzeichnis**, **Tabellenverzeichnis** und **Unterseiten**;
- **Bild**, **Video**, **Zeichnung**, ein Bild **Aus dem Album** oder ein **Kopiertes Bild**;
- **Eintrag einbetten…**: zeigt einen anderen Eintrag vollständig im Text;
- **Neue Aufgabe…** (wird unter der Seite angelegt und als Link oder vollständig gezeigt) und **Aufgabenliste…** (alle Aufgaben mit einem Label oder das Ergebnis einer Abfrage);
- **Ansicht von Einträgen…**: die Einträge eines Typs als Tabelle, Galerie oder Liste, gruppiert nach einem Feld (siehe *Ansichten von Einträgen*);
- **Markdown einfügen…** und **Markdown-Datei aus einem Repository…** (sofern eingeschaltet);
- BPMN-Diagramme sowie Prozesse, Formulare, Entscheidungen, Startformulare und Architekturskizzen einer Prozess-Engine (siehe *Prozess-Engines*).

Bilder lassen sich links oder rechts mit Text daneben oder zentriert platzieren und durch Ziehen in der Grösse ändern; eine Bildunterschrift nummeriert sie im Bildverzeichnis.

### Tabellen

Eine Tabelle wird direkt bearbeitet. Ein Klick in eine Zelle öffnet sie zum Schreiben; Text, der nicht hineinpasst, bricht innerhalb der Zelle um. Tab und Shift+Tab wechseln zur nächsten und zur vorherigen Zelle, Enter zur Zelle darunter, Escape verlässt die Tabelle.

- *Formatieren:* Die Tasten des Textes wirken auch in einer Zelle (Ctrl+B, Ctrl+I, Ctrl+U und die übrigen), ebenso die Schaltflächen über der Tabelle; **Link** (Ctrl+K) verlinkt den markierten Text mit einer Webadresse.
- *Zellen wählen:* Eine Zelle anklicken, dann mit gedrückter Shift-Taste eine weitere: Die Zellen dazwischen sind gewählt. Die Schaltflächen formatieren dann jede gewählte Zelle ganz.
- *Farben:* **Füllung** füllt die gewählten Zellen, **Rand** färbt ihre Ränder, jeweils in einem von acht Farbtönen; **Keine** entfernt die Farbe. Die Farbtöne sind im hellen und im dunklen Design gut lesbar.
- *Grössen:* Den rechten Rand einer Zelle ziehen, um ihre Spalte breiter oder schmaler zu machen, den unteren Rand, um ihre Zeile höher zu machen. Eine Tabelle, die breiter als der Text ist, lässt sich seitwärts scrollen. **Grössen zurücksetzen** überlässt die Grössen wieder dem Text.
- *Zeilen und Spalten:* **+ Zeile** fügt unter der gewählten Zelle eine Zeile ein, **+ Spalte** rechts davon eine Spalte; **− Zeile** und **− Spalte** entfernen die Zeile bzw. Spalte der gewählten Zelle.

Formatierung, Farben und Grössen bleiben im Word- und PDF-Export erhalten; ein Markdown-Export behält die Formatierung.

### Labels, Beziehungen und Dateien

**Labels** sind kurze Begriffe, die Einträge über Räume hinweg gruppieren, etwa `release-2` oder `security`. Man fügt sie in der Seitenleiste hinzu; ein Klick auf ein Label listet alle Einträge damit auf und bietet es **Als Board** an. Ein Label kann eine Seite haben, die es beschreibt (siehe *Seiten von Labels und Sammlungen*).

**Beziehungen** verknüpfen einen Eintrag mit einem anderen unter einem benannten Typ, zum Beispiel *blocks* oder *depends on*: Beziehung und Schlüssel des anderen Eintrags eingeben und **Verknüpfen** wählen. Links und Erwähnungen im Text erscheinen ebenfalls als Beziehungen.

**Dateien** werden mit **Dateien hochladen** angehängt. Wer den Eintrag bearbeiten darf, benennt eine Datei in der Liste mit **Umbenennen** um: Alle Versionen erhalten den neuen Namen, der Dateityp bleibt. Bilder und Zeichnungen aller Einträge erscheinen zudem im Album.

Wer den Eintrag bearbeiten darf, nimmt mit **Ton aufnehmen** auch Ton in seine Dateien auf: Der Browser fragt einmal nach dem Mikrofon, die Aufnahme läuft mit angezeigter Zeit, **Beenden und anhängen** fügt sie als *Aufnahme <Datum> <Uhrzeit>* den Dateien hinzu, **Verwerfen** verwirft sie. Eine Aufnahme endet nach 30 Minuten von selbst und wird angehängt. Wer die Seite während der Aufnahme verlässt, verwirft sie. Die Schaltfläche erscheint nur, wo der Browser aufnehmen kann; dafür muss die Adresse mit `https://` beginnen (oder `localhost` sein). Tondateien – Aufnahmen, MP3, WAV, M4A und Ogg – werden in der Dateiliste abgespielt; sie erscheinen nicht im Album, und nichts wird transkribiert.

### PDF-Dateien lesen

Eine PDF-Datei eines Eintrags öffnet sich im PDF-Leser mit **Lesen** in der Dateiliste oder mit einem Klick auf einen Link zu ihr im Text des Eintrags. Der Leser zeigt die Seiten mit ihrem Text, der sich markieren und kopieren lässt. Die Leiste darüber blättert von Seite zu Seite, vergrössert und verkleinert oder passt auf **Seitenbreite** an, durchsucht das Dokument (Enter für den nächsten Treffer) und bietet **Herunterladen**. Seitlich zeigt *Seiten* kleine Bilder der Seiten, *Inhalt* das Inhaltsverzeichnis des Dokuments, falls es eines hat, und *Markierungen* die darin gesetzten Markierungen. Die Adresse nennt die Seite, sodass ein Link andere zur selben Seite führt.

Wer den Eintrag kommentieren darf, markiert Text: Man wählt ihn auf einer Seite aus und wählt dann eine Farbe, um ihn sofort zu markieren, oder **Mit Notiz markieren**, wählt eine Farbe, schreibt die Notiz und wählt **Markierung speichern**. Eine Markierung ist ein Kommentar zum Eintrag: Sie erscheint unter den Kommentaren des Eintrags mit Datei, Seite und Zitat, und **Im PDF zeigen** führt zu ihr. Markierungen werden in der Liste des Lesers wie in den Kommentaren beantwortet, erledigt und gelöscht; **In den Kommentaren öffnen** führt zum Eintrag. Wird eine neuere Version der Datei hochgeladen, zeigt der Leser die neueste Version und findet jede Markierung an ihrem Text wieder; eine Markierung, deren Text nicht mehr in der Datei steht, sagt das. **Als Markdown exportieren** speichert die Markierungen mit Zitat, Seite und Notiz. Ein PDF mit Markierungen bleibt in seinem Eintrag: Es lässt sich nicht in einen anderen verschieben.
### Felder eines Eintrags

Die Felder eines Eintrags stehen in der Spalte an der Seite und werden gespeichert, sobald sie sich ändern. Neben Text, Zahlen, Datum, Personen und Listen kann ein Feld sein:

- **Eintrag**: ein Verweis auf einen anderen Eintrag, gewählt durch Eintippen einiger Buchstaben seines Schlüssels oder Titels. Er wird mit Schlüssel und Titel angezeigt und zählt als Beziehung: Der andere Eintrag führt ihn unter seinen Beziehungen, die Sternkarte und die Auswirkungen zeigen ihn, und der andere Eintrag lässt sich nicht löschen, solange auf ihn verwiesen wird. Ein Eintrag, den man nicht lesen darf, wird nicht angezeigt.
- **Mehrere aus einer Liste**: beliebige Werte des Felds, angekreuzt.
- **Bewertung**: ein bis fünf Sterne.
- **Datei**: eine der eigenen Dateien des Eintrags, etwa ein Titelbild, als kleines Bild gezeigt. Zuerst die Datei hochladen, dann wählen.

### Verschieben, kopieren und löschen

- **Verschieben …** setzt eine Seite mit ihren Unterseiten unter eine andere Seite oder auf die oberste Ebene des Raums. Innerhalb eines Raums ändern **Nach oben** und **Nach unten** die Reihenfolge.
- **Kopieren …** kopiert eine Seite, auf Wunsch **Mit Unterseiten**.
- **In den Papierkorb** entfernt einen Eintrag aus der Ansicht. Das ist nur möglich, wenn kein anderer Eintrag ausserhalb seines Teilbaums auf ihn verweist. Aus dem Papierkorb lässt er sich mit **Wiederherstellen** zurückholen oder mit **Endgültig löschen** vernichten; das lässt sich nicht rückgängig machen.

### Verlauf und Versionen

Der **Verlauf** listet jede Änderung eines Eintrags mit Person und Zeitpunkt auf. Eine Fassung des Textes zeigt ihre **Änderungen**; **Diese Fassung wiederherstellen** holt einen früheren Text als neue Version zurück. Durch das Wiederherstellen geht nichts verloren.

### Notizen importieren

Raumadministratorinnen und Raumadministratoren holen anderswo geschriebene Notizen mit **Notizen importieren** in den Ansichten des Raums in ihren Raum; die Administration findet dasselbe Formular unter **Administration → Import**. Hochgeladen wird ein ZIP-Archiv mit einem der folgenden Inhalte:

- ein Ordner mit Markdown-Notizen, etwa ein Obsidian-Vault, so wie er ist gezippt, oder der Markdown-Export einer beliebigen Notiz-App, die Markdown-Dateien mit Front Matter schreibt;
- ein Notion-Workspace-Export im Format *Markdown & CSV*.

Was der Import daraus macht:

- jeder Ordner mit Notizen wird zu einer Seite, jede Notiz zu einer Seite unter der Seite ihres Ordners; eine Notiz mit dem Namen des Ordners, `index.md` oder `README.md` ist der eigene Text der Ordnerseite;
- der Titel ist der `title` des Front Matter, sonst die erste Überschrift, sonst der Dateiname; in einem Notion-Export fällt die ID am Ende jedes Namens weg und bezeichnet die Seite;
- Links zwischen Notizen — `[[Notiz]]`, `[[Notiz|Text]]`, `[[Notiz#Überschrift]]` und relative Markdown-Links — werden zu Erwähnungen der importierten Seiten; eine mit `![[Notiz]]` eingebettete Notiz wird ebenfalls zur Erwähnung;
- Bilder im Archiv (`![[bild.png]]` oder ein relativer Bildlink) werden angehängt und angezeigt; Bilder im Web bleiben Links;
- `tags` im Front Matter und Inline-`#tags` werden zu Labels (ein Tag `a/b` wird zum Label `a.b`); ein Front-Matter-Schlüssel mit dem Namen eines Felds des **Typ der Seiten** füllt dieses Feld, ein `status`, den der Workflow des Typs kennt, setzt den Zustand, und alles Übrige bleibt in einer kleinen Tabelle oben auf der Seite erhalten;
- jede Datenbank eines Notion-Exports (eine CSV-Datei) wird zu einer Seite, die ihre Zeilen auflistet, und jede Zeile zu einem Eintrag des **Typ der Zeilen von Datenbanken**, dessen Felder die Spalten nach ihrem Namen füllen.

Beginnen Sie immer mit dem **Probelauf**: Er schreibt nichts und listet auf, wie viele Seiten erstellt oder geändert würden und was sich nicht umwandeln lässt, etwa ein Link auf eine Notiz, die das Archiv nicht enthält, oder ein eingebettetes PDF. **Importieren** schreibt dann die Seiten.

Der **Name des Imports** bezeichnet ihn. Importieren Sie ein neueres Archiv unter demselben Namen, um Änderungen zu übernehmen: Nichts wird doppelt erstellt, eine Seite, deren Notiz sich nicht geändert hat, behält, was hier darin geschrieben wurde, eine geänderte Notiz ersetzt den Text ihrer Seite (der Verlauf behält den alten), Labels kommen nur hinzu, und eine Seite, deren Notiz nicht mehr im Archiv liegt, bleibt erhalten und wird nie gelöscht.

### Markierungen importieren

Die Administration kann unter **Administration → Konnektoren** einen Konnektor der Art Readwise einrichten. Er liest Ihre Markierungen aus Readwise nach seinem Zeitplan, wie jeder Konnektor zuerst mit einem Probelauf: Jedes Buch und jeder Artikel wird zu einem Eintrag des gewählten Typs mit Titel, Autorin oder Autor, Quelladresse und Cover, Labels aus seinen Tags und seinen Markierungen als Zitate mit ihren Notizen und Fundstellen. Ein Lauf liest nur, was sich seit dem letzten geändert hat. Die Markierungen stehen über einer Linie im Text des Eintrags; was Sie unter diese Linie schreiben, bleibt erhalten, wenn die Markierungen aktualisiert werden.
## Weblinks und Erfassen

### Einen Weblink speichern

Eine Seite aus dem Web lässt sich als eigener Eintrag aufbewahren, als *Weblink*: ihre Adresse und was die Seite über sich selbst sagt, etwa Website, Beschreibung, Autorin oder Autor und Erscheinungstag, in seinen Feldern; ihr Bild als Datei; und Ihre eigenen Notizen in seinem Text, wo Kommentare und persönliche Notizen wie auf jeder Seite funktionieren.

Fügen Sie beim Bearbeiten eines Textes eine Adresse auf einer eigenen Zeile ein. Die Blase unter dem Link bietet **Als Weblink-Eintrag speichern** an: Der Eintrag kommt unter die Seite, an der Sie schreiben, oder in einen anderen Raum, und die Zeile mit der Adresse wird zu einer Karte der Seite mit Bild, Titel, Website und Beschreibung. Ein Weblink-Eintrag zeigt dieselbe Karte zuoberst, ebenso jeder Text, der ihn einbettet.

Liest Ihr Server Webseiten (das entscheidet eine Administratorin oder ein Administrator), werden die Felder und das Bild eines Weblinks aus der Seite gefüllt. Auf dem Weblink-Eintrag bringt **Seite neu lesen** sie auf den neuesten Stand und behält ein früheres Bild als frühere Version; **Text der Seite übernehmen** fügt den Haupttext der Seite unter dem Text des Eintrags an, um ihn zu kommentieren und mit persönlichen Notizen zu versehen. Ohne dies behält ein Weblink seine Adresse, und Sie füllen seine Felder selbst aus.

### Von überall erfassen

**Erfassen** speichert eine Seite aus dem Web, ein Zitat oder einen Gedanken, wo immer Sie gerade sind. Es öffnet sich:

- unter **Neu** mit der Karte **Erfassen**;
- von jeder Seite im Web aus mit dem Lesezeichen, das unter **Konto und Passwort** im Kontomenü angeboten wird: Ziehen Sie die Schaltfläche **Erfassen** in die Lesezeichen Ihres Browsers, oder verwenden Sie **Lesezeichen kopieren**. Auf einer Seite öffnet es die Erfassung mit dem Titel der Seite, ihrer Adresse und dem markierten Text;
- auf dem Telefon, indem Sie eine Seite oder einen Text mit Orbithek teilen, sobald Sie es über das Menü des Browsers als App installiert haben.

Die Erfassung zeigt Titel, Adresse und Text, die Sie ändern können, und fragt, wo sie gespeichert werden: als **Weblink** (wenn es eine Adresse gibt), in die **heutige Tagesnotiz** (im Raum, den Sie für Ihre Tagesnotizen gewählt haben), als **neuer Eintrag** eines Typs und in einem Raum Ihrer Wahl oder als **Ergänzung eines Eintrags**, den Sie über die Suche finden. Der Text wird zu Zitaten, die Adresse folgt als ihre Quelle. Gespeichert wird erst mit **Speichern**. Wer sich zuerst anmelden muss, verliert das Geteilte nicht.

## Zusammenarbeiten

### Kommentare

Jeder Eintrag hat an seinem Ende Kommentare. Man schreibt einen Kommentar, erwähnt Personen mit @Name und nutzt **Antworten** und **Erledigt**; erledigte Diskussionen bleiben unter *Erledigt* sichtbar und lassen sich wieder öffnen. Für einen Kommentar zu einer Textstelle markiert man Text innerhalb eines Blocks und wählt **Kommentieren**; der Kommentar zeigt die Stelle, und **Diesen Text auf der Seite zeigen** führt zu ihr zurück.

Das Modul **Kommentare** sammelt Kommentare zu beobachteten Einträgen, die eigenen und jene mit Erwähnungen (*Alle*, *Abonniert*, *Meine*, *Erwähnungen*).

### Beobachten und Benachrichtigungen

**Beobachten** auf einer Seite oder in einem Raum meldet, was dort geschieht: neue Unterseiten, Bearbeitungen, Änderungen von Status und Feldern, Kommentare, Labels und Dateien. Die Glocke in der oberen Leiste führt zu den **Benachrichtigungen**, filterbar nach *Erwähnungen*, *Kommentare* und *Änderungen* oder *Nur ungelesene*. Gemeldet wird auch, wenn jemand einen erwähnt, eine Freigabe-Anfrage beantwortet oder eine selbst gespeicherte Seite wegen schützenswerter Daten eingeschränkt wurde. Unter **Konto und Passwort** legt man die Nachrichtenkanäle fest, etwa eine E-Mail-Adresse.

### Persönliche Notizen

Beim Lesen markiert man eine Stelle und wählt **Notiz**, um eine Notiz dazu anzulegen, die nur man selbst sieht. **Meine Notizen** auf der Startseite listet alle eigenen Notizen mit den zugehörigen Stellen auf.

### Tagesnotizen

Eine Tagesnotiz ist eine eigene Seite für einen Tag: Nur man selbst liest und bearbeitet sie. **Heute** in der oberen Leiste, **Mein Tag** auf der Startseite, Alt+Shift+D oder die Befehlspalette öffnet die Notiz von heute; sie wird beim ersten Öffnen erstellt, im **Raum für Tagesnotizen**, den man einmal wählt, unter den Räumen, in denen man Seiten erstellen darf. Solange kein Raum gewählt ist, steht die Auswahl an der Stelle der Notiz; später ändert man den Raum in der Spalte an der Seite, die sich mit ihrem Pfeil ein- und ausklappen lässt. Bestehende Notizen bleiben, wo sie sind.

Der Text der Notiz wird an Ort und Stelle bearbeitet. **Vorheriger Tag** und **Nächster Tag** wechseln den Tag, und der Monat daneben markiert die Tage mit Notiz; ein Tag ohne Notiz bietet **Notiz für diesen Tag beginnen** an. Neben der Notiz zeigt **Im Kalender an diesem Tag** die Einträge, deren Daten auf diesen Tag fallen, und **An diesem Tag erstellt oder geändert** die lesbaren Einträge, die an diesem Tag erstellt oder zuletzt geändert wurden. **Als Seite öffnen** zeigt die Notiz als Eintrag, mit Kommentaren, Verlauf und Feldern. Tagesnotizen tragen das Label `daily-note`; eine Abfrage nach diesem Label listet sie alle.

### Gleichzeitig bearbeiten

Mehrere Personen können einen Text gleichzeitig bearbeiten. Jeder Block, an dem jemand arbeitet, ist für die anderen gesperrt; die Zeile über dem Text nennt, wer gerade im Text ist und wer welchen Block bearbeitet. Bricht die Verbindung ab, stellt der Editor sie selbständig wieder her.

## Informationen finden

### Suche

Man gibt Wörter aus Titel, Text oder Kommentar oder einen Schlüssel wie `PLAT-12` in das Suchfeld ein. Während der Eingabe erscheinen Vorschläge; **Enter** zeigt alle Treffer, geordnet nach Relevanz (ein bis fünf Sterne). Die Suche findet nur, was man lesen darf.

### Befehlspalette

Ctrl+K (ausser beim Bearbeiten eines Texts, wo Ctrl+K einen Link einfügt) oder Ctrl+Shift+P öffnet die **Befehlspalette**. Man tippt einen Teil eines Titels, eines Schlüssels oder eines Befehls: Sie findet Einträge, bietet unter **Zuletzt geöffnet** die zuletzt geöffneten Einträge an (in diesem Browser gemerkt) und **Befehle** wie einen neuen Eintrag, die Tagesnotiz von heute, ein Modul oder einen Raum, die Abfrage, den Kalender, die helle oder dunkle Darstellung, den Lesemodus und das Handbuch. Die getippten Buchstaben müssen nur in ihrer Reihenfolge vorkommen, so findet `tvh` die *Tagesnotiz von heute*. Pfeiltasten wählen, Enter öffnet, Esc schliesst.

### Abfragen

Eine **Abfrage** wählt Einträge nach ihren Feldern aus. Man öffnet sie aus den Suchergebnissen (**Stattdessen die Felder der Einträge abfragen**). Bedingungen werden mit AND, OR und NOT verbunden; Klammern fassen sie zusammen:

- Vergleiche mit `=`, `!=`, `<`, `<=`, `>`, `>=`; `~` findet jedes Wort eines Werts; `IN (a, b)` ist einer aus einer Liste.
- `IS EMPTY` und `IS NOT EMPTY` finden Einträge ohne bzw. mit Wert.
- Ein Datum schreibt man `2026-10-01`; `-7d`, `2w`, `+12h` und `-1y` zählen ab jetzt; `now()`, `startOfWeek()` und ähnliche Funktionen stehen zur Verfügung.
- `currentUser()` steht für die eigene Person.
- Ein Eintragsfeld nimmt den Schlüssel eines Eintrags, etwa `epic = PLAT-3`; `platforms = web` findet Einträge, die *web* unter mehreren Werten haben; eine Bewertung vergleicht wie eine Zahl, etwa `rating >= 4`.
- `ORDER BY` sortiert, `ASC` oder `DESC`.

Beispiele:

```
type = task AND assignee = currentUser() AND finished = false ORDER BY due
updated >= -7d ORDER BY updated DESC
text ~ "release" AND label IN (web, security)
```

Ctrl+Enter führt eine Abfrage aus. Die Ergebnisse lassen sich als CSV exportieren.

### Ansichten eines Raums

Ein Raum bietet mehrere Ansichten: seinen Seitenbaum, **Alle Einträge**, ein **Board** je Typ mit Workflow, **Ansichten** seiner Einträge, den **Kalender**, die **Roadmap** und, je nach Rolle, **Governance** und **Linkprüfung**. **Beobachten** in einem Raum meldet alles, was darin geschieht.

### Ansichten von Einträgen

**Ansichten** in einem Raum, oder **Ansicht eines Typs…** daneben, zeigt die Einträge, die Sie lesen dürfen, als **Tabelle**, als **Galerie** von Karten oder als **Liste** kompakter Zeilen. Wählen Sie den **Raum** (oder alle Räume), den **Typ** und nach Wunsch eine **Abfrage** in der Abfragesprache, dann **Anwenden**. **Gruppieren nach** ordnet die Einträge unter Überschriften mit ihrer Anzahl: nach Status, nach Label oder nach einem Feld mit einer Auswahl, einer Person oder ja und nein; ein Klick auf eine Überschrift klappt die Gruppe zu. **Sortierung** ordnet sie nach einer Spalte, ebenso ein Klick auf eine Spaltenüberschrift, ein zweiter Klick umgekehrt. **Spalten** wählt, was die Tabelle zeigt; die Karten einer Galerie zeigen das erste Bild des Texts oder die Datei eines Dateifelds, mit dem Titel und den gewählten Feldern.

Wo Sie einen Eintrag bearbeiten dürfen, ändert der Stift neben einer Zelle sie direkt: den Titel, ein Feld oder den Status, über einen Übergang des Workflows. Die Adresse hält die ganze Ansicht fest, so zeigt ein Link sie anderen, so weit sie die Einträge lesen dürfen.

In einem Text fügt **Einfügen…** → **Ansicht von Einträgen…** eine solche Ansicht ein; **Ansicht ändern…** ändert sie beim Bearbeiten, und **Als Ansicht öffnen** zeigt sie auf einer eigenen Seite. Alle sehen nur die Einträge, die sie lesen dürfen.

### Seiten von Labels und Sammlungen

Ein Label kann eine Seite haben, die sagt, worum es geht. In der Liste der Einträge eines Labels erstellt **Dieses Label beschreiben** diese Seite, vom Typ *topic*, in einem Raum Ihrer Wahl; danach zeigt die Liste des Labels sie oben mit **Seite lesen**. Haben mehrere Räume eine Seite zum selben Label, sehen Sie die älteste, die Sie lesen dürfen.

Die Seite eines Labels zeigt die Einträge mit ihrem Label als Ansicht: **Tabelle**, **Galerie** oder **Liste**, gruppiert und sortiert nach Wahl. Mit **Von Hand festgelegte Reihenfolge** zieht, wer die Seite bearbeiten darf, einen Eintrag vor einen anderen oder verschiebt ihn mit den Pfeilen (Alt+↑ und Alt+↓); später hinzugekommene Einträge folgen nach den geordneten, und Einträge, die Sie nicht lesen dürfen, behalten ihren Platz, ohne gezeigt zu werden. **Von Hand festgelegte Reihenfolge vergessen** listet sie wieder in der Reihenfolge ihrer Erstellung. Eine solche Seite mit ihrer Reihenfolge ist eine Sammlung: **Zu Sammlung hinzufügen…** bei einem Eintrag gibt ihm das Label einer Seite Ihrer Wahl.

## Aufgaben

### Die Aufgabenliste

Das Modul **Aufgaben** listet die offene Arbeit in den eigenen Räumen auf: Einträge vom Typ *task* sowie jedes Typs mit Workflow und einem Feld *assignee* oder *due*. Die Reiter zeigen **Mir zugewiesen**, **Nicht zugewiesen** und **Alle offenen**; Filter schränken nach Raum, Status, Person, Fälligkeit und Labels ein. **Übernehmen** weist eine Aufgabe sich selbst zu, **Zurückgeben** gibt sie frei. Überfällige Aufgaben sind markiert.

Der vorgefertigte Typ *task* hat neben dem Datum *due* ein Datum *start*. Eine neue Aufgabe beginnt heute: Beim Erstellen wird ihr Startdatum mit dem aktuellen Tag vorbelegt, den man im Formular ändern oder leeren kann. Dasselbe gilt für jedes Datumsfeld, dessen Name einen Beginn bezeichnet (*start*, *begin*, *kickoff* und ähnliche); eine im Kalender erstellte Aufgabe übernimmt den dort gewählten Tag. Kalender und Roadmap zeigen eine Aufgabe mit beiden Daten als Balken vom Beginn bis zur Fälligkeit. Ein Startdatum ist keine Frist: Auf einem Kanban wird ein vergangenes Startdatum nicht als überfällig markiert. Bereits angelegte Typen erhalten das Feld, wenn eine Administratorin oder ein Administrator auf der Startseite die neuen Felder der vorgefertigten Typen hinzufügt.

### Boards und Kanbans

Ein **Board** zeigt die Einträge eines Typs in einem Raum als Spalten ihrer Workflow-Status. Das Ziehen einer Karte in eine andere Spalte führt den Übergang des Workflows aus.

Ein **Kanban** fasst die Aufgaben eines Themas zusammen: Unter *Aufgaben → Kanbans* wählt man **Neues Kanban**, gibt einen Namen, den Typ der Karten und optional ein Label an. Jede Karte darauf trägt das Label. In den **Board-Einstellungen** wählt man:

- die **Spalten**: Status-Spalten führen eine Karte durch den Workflow, eigene Spalten behalten ihren Status;
- eine **Höchstzahl** Karten je Spalte, die eine überfüllte Spalte markiert;
- die Felder **Auf den Karten angezeigt**, die **Farbe der Karten** nach einem Auswahlfeld und **Swimlanes** nach einem Personen- oder Auswahlfeld.

Karten werden mit **Neue Karte** hinzugefügt, für ihre Details geöffnet und mit **Vom Kanban nehmen** entfernt. **Boards nach Label** zeigt die Aufgaben jedes Labels als Board, ohne ein Kanban anzulegen.

### Roadmap und Gantt-Diagramm

Die **Roadmap** zeigt Einträge mit Datumsfeldern auf einer Zeitachse: als **Roadmap nach Label** in Bahnen oder als **Gantt-Diagramm** mit einer Zeile je Eintrag. Man wählt Raum, Typen, die Felder **Beginn** und **Ende**, die Labels und den **Massstab** (Wochen, Monate, Quartale). Einträge, die auf unerledigte Einträge warten, sind markiert.

**Exportieren** speichert das Diagramm als Bild, als **PNG** oder **JPEG**, mit allen Zeilen und der ganzen Zeitachse, auch dem, was ausserhalb des sichtbaren Bereichs liegt. Das Bild zeigt, was man sieht, im aktuellen Erscheinungsbild, und ist nach der Ansicht und dem Tag benannt, etwa *Gantt-Diagramm PLAT 2026-11-12.png*. Ein Diagramm, das für ein Bild zu gross ist, das der Browser zeichnen kann, wird mit einem Hinweis abgelehnt; man schränkt es nach Labels, Typ oder Raum ein oder wählt einen gröberen Massstab.

### Kalender

Der **Kalender** zeigt Einträge mit Datumsfeldern als **Monat**, **Woche** oder **Agenda**. **Filter** wählt Raum, Typ, Zuständigkeit, Status, Wörter des Textes und Labels; ein Satz über dem Kalender beschreibt, was gezeigt wird. Unter **Erweitert** wählt man die Datumsfelder und eine zusätzliche Abfrage.

- Wo man bearbeiten darf, zieht man einen Eintrag auf einen anderen Tag oder verschiebt ihn mit Shift und einer Pfeiltaste. In der Wochenansicht behält eine auf eine andere Stunde verschobene Besprechung ihre Dauer.
- Ein wiederkehrender Eintrag fragt, ob **Nur dieses Vorkommen** oder **Die ganze Serie** verschoben wird; die Taste Entf sagt ein Vorkommen ab.
- Wo man in einem Raum bearbeiten darf, erstellt ein Doppelklick auf einen Tag oder in der Wochenansicht auf eine Stunde dort einen neuen Eintrag; **Neuer Eintrag** tut dasselbe für heute. Der Dialog fragt nach Typ, Raum, Titel und den Feldern, die der Typ verlangt, und füllt das Datum aus der angeklickten Stelle aus. **Erstellen** bleibt im Kalender mit dem neuen Eintrag im Fokus; **Erstellen und öffnen** öffnet ihn.
- **Ansicht speichern…** legt eine Ansicht unter einem Namen ab. Sind Kalender-Feeds eingeschaltet, liefert **Abonnieren (iCal)** eine Adresse für ein Kalenderprogramm. Wer diese Adresse kennt, sieht die Einträge ohne Anmeldung; **Neue Adresse** und **Feed beenden** ziehen sie zurück.
- Erlaubt es die Administration, zeigt **Andere Kalender** die eigenen Kalender anderer Dienste neben denen von Orbithek: iCalendar-Adressen, Google-Konten und CalDAV-Konten wie Apple iCloud. Sie werden nur lesend und nur der eigenen Person gezeigt.

## Album und Bilder

### Ansichten des Albums

Das **Album** zeigt die Bilder und Zeichnungen aller Seiten und Aufgaben, die man lesen darf. Man wählt, wie es sie zeigt:

- **Ordner**: Bilder in Ordnern eines Raums; eine **Sammlung** trägt Bilder mit bestimmten Labels aus allen Räumen zusammen;
- **Zeitachse**: nach dem Monat der Aufnahme;
- **Favoriten**: die mit dem Stern markierten Bilder;
- **An diesem Tag**: Bilder, die an diesem Tag in früheren Jahren aufgenommen wurden;
- **Karte**: die Aufnahmeorte;
- **Duplikate**: Bilder, die dieselbe Datei sind oder gleich aussehen (siehe *Duplikate*).

Man sucht nach Name, Label, Schlüssel oder Titel und schränkt nach Art, Raum und Label ein. Die Suche nimmt auch ein Datum und findet die Bilder, die dann aufgenommen wurden, oder, wenn ein Bild keine Zeit nennt, dann hinzugefügt wurden: ein Jahr (*2026*), einen Monat (*11.2026*, *2026-11*, *November 2026*), einen Tag (*10.11.2026*, *2026-11-10*, *10. November 2026*) oder einen Tag oder Monat ohne Jahr (*10.11.*, *November*), der in jedem Jahr gefunden wird.

In der Ansicht **Ordner** steht über dem Inhalt der Pfad zum offenen Ordner, daneben, was den Ordner selbst ändert: **Neuer Ordner**, **Neue Sammlung** und **Ordner löschen**, in einer Sammlung **Regel ändern** und **Sammlung löschen**. Darunter folgen die Wege, Bilder hinzuzufügen, dann die Ordner und direkt über den Bildern ihre Zahl, **Mehrere auswählen** und **N Bilder präsentieren**.

### Hochladen und importieren

Bilder werden dort hinzugefügt, wo man gerade ist, in der Ansicht **Ordner**:

- *In einem Ordner* wählt man **Bilder hochladen**, legt Bilder und ZIP-Archive auf dem Album ab oder fügt ein Bild mit Strg+V ein, etwa ein Bildschirmfoto. Ein leerer Ordner zeigt dafür eine grosse Fläche. **Aus dem Album hinzufügen…** holt Bilder, die schon im Album sind, in den Ordner: Mit **Kopieren** bleibt das Original, wo es ist; mit **Verschieben** wandert das Bild mit seinen Versionen, ausser man darf es nicht ändern oder ein Text zeigt es. Das Album merkt sich die Wahl.
- *In einer Sammlung* gelten dieselben Wege. Eine Sammlung hält keine eigenen Dateien; das Album fragt deshalb, in welchem Ordner die neuen Bilder abgelegt werden, oder legt einen **Neuer Ordner…** an. Die Sammlung merkt sich den Ordner für alle, die Bilder hinzufügen, und schlägt ihn beim nächsten Mal vor; ändern kann ihn nur, wer die Sammlung bearbeiten darf. Jedes Bild erhält die Labels der Sammlung und erscheint so in ihr. **Aus dem Album hinzufügen…** gibt Bildern, die schon im Album sind, die Labels der Sammlung, ohne sie zu kopieren.
- *Ganz oben* im Album fragt das Hochladen, Ablegen oder Einfügen ebenfalls nach dem Ordner, der sich danach öffnet.

Eine PowerPoint-Präsentation wird zu einem Ordner von Folien mit ihren Notizen (siehe *Eine PowerPoint-Präsentation importieren*). Aus Foto-Apps exportierte Bilder werden im Browser entpackt. Sofern eingeschaltet, kommen Fotos auch **Aus Google Fotos** oder **Aus geteiltem iCloud-Album**. HEIC-Fotos werden nicht übernommen; sie sind vorher als JPEG zu exportieren. Videos aus Google Fotos und geteilten iCloud-Alben kommen ebenfalls, mit dem Bild, das der Dienst von ihnen hat, als Vorschaubild.

Der **Ort beim Import** eines Ordners bestimmt, was mit dem Aufnahmeort eines Fotos geschieht: **Für alle behalten**, **Nur für mich** oder **Aus der Datei entfernen**. Pro Bild lässt sich der Ort später nur für sich behalten oder allen zeigen.

Nennt ein Bild keinen oder einen falschen Ort, setzt ihn, wer das Bild bearbeiten darf, von Hand als **Ort**: im Betrachter oder für mehrere ausgewählte Bilder auf einmal. Die Eingabe eines Namens wie *Zermatt* schlägt Orte vor, sofern die Administration die Vorschläge nicht ausgeschaltet hat; **Auf der Karte wählen** setzt den Ort mit einem Klick auf die Karte. **Ort entfernen** löscht ihn.

Nennt ein Bild keine oder eine falsche Aufnahmezeit, etwa ein Scan oder ein Foto einer Kamera mit falsch gestellter Uhr, setzt sie, wer das Bild bearbeiten darf, von Hand als **Aufnahmezeit**. Im Betrachter werden Datum und Uhrzeit sowie, falls bekannt, die **Abweichung von UTC** eingegeben und mit **Zeit setzen** übernommen; **Zurück zur Zeit der Datei** stellt die Zeit wieder her, welche die Datei selbst nennt. Für mehrere ausgewählte Bilder bietet **Zeit setzen…** die Wahl **Setzen auf**, die allen dieselbe Zeit gibt, und **Verschieben um**, die jedes Bild um dieselben Tage, Stunden und Minuten später oder früher legt; Bilder ohne Zeit werden nicht verschoben. Das Album, die Zeitachse und *An diesem Tag* verwenden danach die von Hand gesetzte Zeit, und jeder Download, jedes ZIP-Archiv und jede Kopie trägt sie in den EXIF-Daten des Bildes, während die gespeicherte Datei unverändert bleibt. Ein Video behält in seiner Datei die eigene Zeit.

### Videos

Videos kommen auf denselben Wegen ins Album wie Bilder: hochgeladen, abgelegt, eingefügt, in einem ZIP-Archiv oder aus Google Fotos und geteilten iCloud-Alben, als MP4, WebM oder MOV (iPhone), je bis 2 GB, sofern die Administration keine andere Grenze gesetzt hat. Das Album behandelt sie danach wie Bilder: in Ordnern und Sammlungen, in der Zeitachse und auf der Karte, als Favoriten und Erinnerungen, mit Labels, Umbenennen, Verschieben, Löschen und dem ZIP-Download.

- Beim Hochladen nimmt der Browser ein Bild aus dem Video; das Album zeigt es auf der Karte zusammen mit der Dauer des Videos. Ein Video, das der Browser nicht abspielen kann, behält stattdessen ein Symbol.
- Aufnahmezeit und -ort, Dauer und Kamera werden aus dem Video selbst gelesen; **Ort beim Import** wirkt wie bei Fotos.
- Ein Klick spielt das Video im Betrachter ab. Ob ein MOV-Video abspielt, hängt vom Browser ab: Safari spielt es ab, andere Browser oft nur, wenn es als H.264 statt HEVC aufgenommen wurde. Sonst wählt man **Herunterladen**.
- Der Filter **Videos** zeigt nur Videos. Als PNG oder JPEG werden sie nicht exportiert.
- Beim Präsentieren ist ein Video eine Folie: Es zeigt sein Vorschaubild, bis die Leertaste oder ein Klick es startet, im Präsentationsfenster und nur dort mit Ton. Die Leertaste hält es wieder an; die Pfeiltasten gehen weiter. Es lässt sich nicht vergrössern und nicht mit dem Laserpointer zeigen.

### Duplikate

Bevor Bilder und Videos hochgeladen, abgelegt oder eingefügt werden, prüft der Browser, ob sie schon im Album sind, in allen Ordnern, die man lesen darf:

- **Dieselbe Datei**: ein Bild mit genau demselben Inhalt;
- **Sieht gleich aus**: dasselbe Bild erneut gespeichert, kleiner, in einem anderen Format oder ohne seinen Ort. Die Proportionen müssen gleich sein; ein anders zugeschnittenes Bild ist kein Duplikat.

Doppelt gewählte Dateien zählen ebenfalls. Werden Duplikate gefunden, listet ein Dialog jede Datei mit den Bildern, denen sie entspricht, und deren Ordnern. Angekreuzte Dateien werden übersprungen: **Angekreuzte überspringen** fügt die übrigen hinzu, **Alle hinzufügen** fügt alles hinzu, **Abbrechen** fügt nichts hinzu. Videos werden nur als Dateien verglichen. Kopien, die mit **Aus dem Album hinzufügen…** entstanden sind, sind keine Duplikate ihres Originals. Die Prüfung setzt eine sichere Verbindung (HTTPS) voraus; ohne sie werden Dateien ungeprüft hinzugefügt.

Die Ansicht **Duplikate** zeigt Gruppen von Bildern, die dieselbe Datei sind oder gleich aussehen, die neueste Gruppe zuerst, soweit der Raumfilter sie einschränkt. Dort wählt man Bilder aus und löscht sie wie in jeder Ansicht. **Duplikate löschen…** löscht sie nach einer Warnung alle auf einmal: Von jeder Menge derselben Datei bleibt ein Bild, eines, das ein Text zeigt oder das man nicht ändern darf, sonst das grösste, sonst das älteste. Kreuzt man das Feld an, werden auch Bilder gelöscht, die nur gleich aussehen, sodass von jeder Gruppe ein Bild bleibt. Gelöschte Bilder lassen sich nicht wiederherstellen. Bilder, die vor dieser Prüfung hinzugefügt wurden, werden verglichen, sobald das Album zu ihnen kommt: Die Ansicht nennt, wie viele noch warten, und vergleicht bei jedem Öffnen weitere.

### Gesichter und Personen

Öffnet man ein Bild, das man bearbeiten darf, sucht der Browser darauf nach Gesichtern; **Gesichter finden** im Betrachter sucht erneut. Unter **Personen auf dem Bild** hat jedes Gesicht ein Feld für den Namen: einen Namen eintippen oder einen der bereits vergebenen wählen. **Entfernen** nimmt ein Gesicht weg. Wer das Bild lesen darf, sieht die Namen und einen Rahmen um jedes Gesicht.

Die Ansicht **Personen** listet alle benannten Personen mit ihrem Gesicht und der Zahl der Bilder, die sie zeigen; ein Klick öffnet das Album dieser Person, das sich auch als Folien präsentieren lässt. **Gesichter in allen Bildern finden** sucht auf jedem angezeigten Bild nach Gesichtern, das man bearbeiten darf und das noch nie durchsucht oder, bei eingeschalteter Wiedererkennung, noch nicht erkannt wurde. Auch auf hochgeladenen Bildern wird nach dem Hochladen nach Gesichtern gesucht.

Gesichter werden im eigenen Browser gefunden; nichts wird anderswohin gesendet. Hat die Administration **Gesichter wiedererkennen** eingeschaltet, berechnet der Browser zu jedem Gesicht zusätzlich Zahlenwerte, mit denen das Album für ein Gesicht, das bereits benannten Gesichtern ähnelt, einen Namen vorschlägt, etwa *Ada?*. Diese Werte sind biometrische Daten: Man schaltet das nur ein, wenn die abgebildeten Personen einverstanden sind. Beim Ausschalten werden sie alle gelöscht; die Namen bleiben. Bilder, die vor dem Einschalten durchsucht wurden, werden erst erkannt, wenn sie erneut durchsucht werden: Nach dem Einschalten weist die Administration auf **Gesichter in allen Bildern finden** hin, und ein im Betrachter geöffnetes Bild wird von selbst erneut durchsucht.

Ist die Wiedererkennung eingeschaltet, zeigt das Album einer Person zusätzlich **Ist das Ada?**: Gesichter auf anderen Bildern, die man bearbeiten darf und die Ada mehr ähneln als allen anderen benannten Personen, die ähnlichsten zuerst. Man klickt die Gesichter an, die Ada zeigen, oder wählt **Alle wählen** und klickt die falschen wieder ab, dann **Gewählte bestätigen**. Nicht gewählte Gesichter bleiben unbenannt und werden später erneut vorgeschlagen. Vorgeschlagen werden nur Bilder, deren Gesichter bei eingeschalteter Wiedererkennung gesucht wurden.

### Ordner, die einen Fileshare zeigen

Ein Administrator kann einen Album-Ordner an einen Fileshare binden: ein Verzeichnis des Servers, etwa einen eingebundenen Samba-Share, oder einen S3-Bucket.
Der Ordner zeigt dann die Bilder des Shares und liest ihn nach einem Zeitplan neu.
Darunter enthält für jedes Verzeichnis des Shares ein eigener Ordner dessen Bilder; diese Ordner folgen dem Share und werden nicht von Hand erstellt, verschoben oder gelöscht, und ein von Hand geänderter Titel wird zurückgesetzt.
Der Share bleibt massgebend: Das Wiki behält von jedem Bild eine Vorschau, das Original bleibt, wo es liegt.

Ein solches Bild ist mit **Auf einem Fileshare** markiert.
**Original öffnen** öffnet es dort, wo es liegt, mit dem eigenen Zugriff auf den Share; der Share entscheidet, ob es sich öffnet.
Man versieht es mit Labels, benennt die Personen darauf, setzt Ort und Zeit und markiert es als Favorit wie jedes Bild.
Man benennt es nicht um, verschiebt oder löscht es nicht und fügt einem solchen Ordner keine Bilder hinzu: Man ändert die Dateien auf dem Share, und das nächste Lesen folgt.
Eine auf dem Share verschobene Datei behält, was auf ihr gesetzt wurde, solange ihr Inhalt gleich bleibt.
Eine vom Share verschwundene Datei verschwindet aus dem Album; kommt sie innert 30 Tagen zurück, kehrt sie mit ihren Labels und Namen zurück.
Wer ein solches Bild kopiert oder exportiert, erhält seine Vorschau.
Ein RAW einer Kamera zeigt als Vorschau das Bild, das die Kamera darin eingebettet hat, ein WebP sein Bild.
Ein Bild in einem Format, von dem der Server kein Bild zeigt, etwa HEIC eines Telefons, hat statt einer Vorschau ein Symbol; es bietet nur **Original öffnen** an, ebenso ein Video eines Shares.
Erlaubt ein Administrator einem S3-Share signierte Links, öffnet **Original öffnen** das Original über einen Link, den der Server für 15 Minuten signiert, unabhängig von den Berechtigungen des Shares.

### Einen Ordner verschieben

**Ordner verschieben** in einem offenen Ordner verschiebt ihn mit seinen Ordnern, Bildern und Videos in einen anderen Ordner oder auf die oberste Ebene eines Raums. Den Ort unter **Verschieben nach…** wählen und **Verschieben** klicken; der Pfad führt danach zum Ordner an seinem neuen Ort. Ein Ordner lässt sich auch auf einen daneben aufgeführten Ordner oder auf einen übergeordneten Ordner im Pfad ziehen; auf **Alle Bilder** gezogen, kommt er auf die oberste Ebene seines Raums. Wird der Ordner in einen anderen Raum verschoben, fragt das Album zuerst nach: Danach entscheidet dieser Raum, wer ihn sehen und ändern darf. Verschieben darf einen Ordner nur, wer ihn und alles darin ändern und am Ziel etwas erstellen darf; in sich selbst oder einen Ordner darunter lässt er sich nicht verschieben.

### Einen Ordner löschen

**Ordner löschen** in einem offenen Ordner verschiebt ihn mit seinen Ordnern, Bildern und Videos in den Papierkorb, nachdem gesagt wurde, wie viel mitgeht. Seine Seite stellt ihn mit **Wiederherstellen** wieder her. Ein Ordner wird nicht gelöscht, solange etwas ausserhalb, etwa eine Seite, noch verwendet, was darin ist; das Album nennt es.

**Sammlung löschen** in einer offenen Sammlung verschiebt nur die Sammlung in den Papierkorb. Ihre Bilder und Videos bleiben, wo sie sind, mit ihren Labels; ihre Seite stellt sie mit **Wiederherstellen** wieder her.

### Eine PowerPoint-Präsentation importieren

Eine Präsentation wird zu einem Ordner von Folien, bereit zum Präsentieren mit ihren Foliennotizen. Im Album den Ordner öffnen, in den die Präsentation gehört, **Bilder hochladen** wählen und die *.pptx*-Datei wählen – oder sie einfach auf dem Ordner ablegen.

Im offenen Ordner entsteht ein Ordner mit dem Namen der Präsentation und öffnet sich. Jede Folie wird zu einem Bild, benannt nach ihrer Stelle und ihrem Titel, etwa *03 Unsere Ziele.png*, sodass das Album die Reihenfolge der Folien behält; die Notizen der Folie werden zu den Foliennotizen des Bildes. Ausgeblendete Folien werden weggelassen, wie eine Bildschirmpräsentation sie weglässt.

Eine Präsentationsdatei enthält keine Bilder ihrer Folien, deshalb zeichnet der Browser jede Folie selbst: Hintergründe, Text mit Aufzählungszeichen und Nummerierung, Formen, Bilder, Tabellen und SmartArt, in den Farben und Schriften der Präsentation. Das Ergebnis kommt dem Original nahe, ist aber nicht immer gleich: Eine Schrift, die auf dem Computer fehlt, wird durch eine ähnliche ersetzt, sodass Zeilen anders umbrechen können, und Effekte wie Schatten fehlen. Diagramme, eingebettete Objekte und Bilder in Formaten, die der Browser nicht anzeigen kann (etwa EMF), erscheinen als graue Flächen; das Album nennt dann, wie viele Folien sie enthalten.

Für exakte Folien exportiert man sie zuerst aus PowerPoint: **Datei › Exportieren › Dateityp ändern › PNG** (oder JPEG), **Alle Folien**. Dann die *.pptx*-Datei zusammen mit all diesen Bildern wählen oder zusammen mit einem ZIP-Archiv davon. Das Album übernimmt dann die Bilder, statt zu zeichnen, und ordnet sie den Folien über die Zahlen in ihren Namen zu, etwa *Folie1.PNG* oder *Slide1.PNG*. Passen sie nicht, wird nichts übernommen und das Album nennt, wie viele Folien und Bilder es gefunden hat.

Nur die Bilder und die Notizen gelangen auf den Server, nicht die Präsentationsdatei. Animationen, Übergänge, Videos und die Kommentare der Präsentation werden nicht übernommen; *.ppt*-Dateien älterer Versionen sind zuerst als *.pptx* zu speichern.

### Mit Bildern arbeiten

Ein Klick öffnet ein Bild mit seinen Angaben: Zeitpunkt und Ort der Aufnahme, Kamera, Belichtung, Labels und der zugehörige Eintrag. Die Karte eines Bildes und ihr Menü bieten:

- **Umbenennen**: gibt dem Bild einen anderen Namen (siehe *Bilder und Dateien umbenennen*);
- **Kopieren**: um es mit Ctrl+V in eine Seite einzufügen; die Kopie kennt ihr Original und meldet, wenn es sich geändert hat;
- **Exportieren**: PNG, JPEG oder bei Zeichnungen SVG;
- **Einfügen** in die gerade bearbeitete Seite, **Löschen** und ältere Versionen.

**Mehrere auswählen** wählt viele Bilder auf einmal aus, um Labels hinzuzufügen, Favoriten zu markieren, sie in einen Ordner zu verschieben, sie mit **Als ZIP herunterladen** herunterzuladen oder sie mit **Löschen** auf einmal zu löschen; **Alle angezeigten auswählen** wählt alle gezeigten Bilder.

In den Ordnern verschiebt man ein Bild, indem man es auf einen Ordner zieht: auf einen Ordner, der unter dem geöffneten aufgeführt ist, oder auf einen übergeordneten Ordner im Pfad oben. Ein ausgewähltes Bild nimmt alle ausgewählten Bilder mit. Ein verschobenes Bild bleibt dasselbe Bild: Alle Versionen, Labels, Elementnotizen, der Ort und die Favoriten-Markierungen gehen mit, und Kopien in Seiten finden ihr Original weiterhin. Ein Bild, das man nicht ändern darf, das ein Text zeigt oder das geschützt ist, und eine Zeichnung mit Kommentaren bleiben, wo sie sind; das Album nennt sie mit dem Grund. Ohne Maus wählt man die Bilder aus und nutzt **In Ordner verschieben…**, das sie auf dieselbe Weise verschiebt.

### Bilder und Dateien umbenennen

Wer ein Bild bearbeiten darf, benennt es im Album um: das Menü **…** der Karte öffnen und **Umbenennen** wählen. Der Name auf der Karte wird zum Eingabefeld. **Enter** oder ein Klick daneben speichert; **Escape** behält den bisherigen Namen. In der Dateiliste einer Seite tut **Umbenennen** neben **Löschen** dasselbe für jede Art von Datei, auch für Dokumente und Videos.

- Man tippt nur den Namen: Der Dateityp bleibt. Eine Zeichnung bleibt eine Zeichnung, und aus *IMG_0042.JPG* wird mit *See* der Name *See.JPG*.
- Alle Versionen der Datei erhalten den neuen Namen, auch die älteren Versionen einer Zeichnung.
- Kopien des Bildes in anderen Seiten behalten ihren Namen, ebenso die Bildunterschriften in einem Text.
- Wird eine Zeichnung neu gezeichnet, behält die neue Version den Namen.

### Bilder präsentieren

Das Album präsentiert die Bilder, die es gerade zeigt, als Folien: einen Ordner oder eine Sammlung öffnen, nach Wunsch mit der Suche oder einem Label eingrenzen und **N Bilder präsentieren** wählen. Die Folien eines Ordners oder einer Sammlung folgen seiner Reihenfolge der Folien (siehe *Folien von Hand ordnen*); noch nicht eingereihte Bilder folgen danach, nach Namen, Zahlen nach ihrem Wert, also *2 Plan* vor *10 Ausblick*.

Der Klick öffnet ein zweites Fenster, das **Präsentationsfenster**, für den Projektor oder einen zweiten Bildschirm. Das Fenster des Albums wird zum **Referentenfenster**. Es zeigt die aktuelle und die nächste Folie, die Foliennotizen, die Zeit seit dem Start und die Uhrzeit sowie eine Leiste aller Folien; ein Klick auf eine Folie in der Leiste springt dorthin. Hat der Browser das zweite Fenster blockiert, Pop-ups für diese Seite erlauben und **Präsentationsfenster** wählen. Im Präsentationsfenster füllt **Vollbild** oder **F** den Bildschirm.

Beide Fenster gehorchen denselben Tasten und Schaltflächen:

| Was | Wie |
|---|---|
| Nächste oder vorherige Folie | Pfeiltasten, Bild auf/ab, Leertaste, die Schaltflächen; ein Klick ins Präsentationsfenster blättert weiter |
| Erste oder letzte Folie | Pos1, Ende |
| Zoom | + und −, die Schaltflächen oder das Mausrad dort, wohin es zeigt; eine vergrösserte Folie mit der Maus verschieben; **0** oder ein Doppelklick zeigt die ganze Folie |
| Laserpointer | **L** oder **Laser**, dann mit der Maus zeigen; der rote Punkt erscheint in beiden Fenstern |
| Schwarzer Bildschirm | **B** oder **Schwarz**, nochmals für die Folie |

Präsentations-Fernbedienungen, die Bild auf und Bild ab senden, funktionieren ebenfalls. **Beenden** schliesst das Präsentationsfenster und kehrt ins Album zurück.

**Foliennotizen** gehören zu einem Bild; auf seiner Karte im Album heissen sie Elementnotizen. Man schreibt sie dort mit **Elementnotizen hinzufügen** oder im Referentenfenster mit **Notizen bearbeiten**. Wer das Bild bearbeiten darf, schreibt sie; wer das Bild sieht, liest sie. Die Notizen bleiben bei allen Versionen einer Zeichnung erhalten.

Beide Fenster müssen im selben Browser auf demselben Computer laufen. Personen an anderen Orten folgen über Bildschirmfreigabe.

### Folien von Hand ordnen

Ein Ordner oder eine Sammlung behält eine eigene Reihenfolge der Folien, für alle gleich, die ihn lesen. In einem geöffneten Ordner oder einer Sammlung **Reihenfolge der Folien** statt **Neueste zuerst** wählen: Das Album zeigt die Bilder in der Reihenfolge, in der sie präsentiert werden. Wer den Ordner oder die Sammlung bearbeiten darf, zieht ein Bild auf ein anderes: Auf dessen linke Hälfte fallen gelassen, kommt es davor, auf die rechte Hälfte danach. Ausgewählte Bilder werden gemeinsam verschoben. Ohne Maus bietet das Menü eines Bildes **Eine Stelle nach vorne** und **Eine Stelle nach hinten**.

Im Referentenfenster werden die Folien der Leiste ebenso gezogen; die Reihenfolge wird sofort gespeichert, und das Präsentationsfenster folgt.

Später hinzugefügte Bilder folgen nach den eingereihten, nach Namen; eine neue Version einer Zeichnung behält ihren Platz. **Nach Namen ordnen** verwirft die von Hand gesetzte Reihenfolge. Um ein Bild hinter das letzte gezeigte zu setzen, während weitere folgen, zuerst **Mehr anzeigen** wählen.

## Zeichnungen und Whiteboards

### Zeichnungen

**Einfügen… → Zeichnung** erstellt eine Zeichnung im Text. Der Zeichnungseditor bietet Paletten für ArchiMate, BPMN, Netzwerk- und IT-Symbole. Eine Zeichnung wird als Bild ihrer Seite gespeichert, mit allen Versionen.

### Live-Whiteboards

Jede Zeichnung lässt sich mit **Als Whiteboard öffnen** öffnen: Bis zu 20 Personen und KI-Agenten zeichnen gleichzeitig darauf, wobei jedes Element gesperrt ist, solange jemand daran arbeitet. Ein Whiteboard bietet farbige Notizen (**Notiz**), eine **Abstimmung** mit einer Anzahl Stimmen pro Person (das Ergebnis wird als Kommentar festgehalten), einen **Timer** und **Präsentieren**, dem die anderen Bild für Bild folgen. Kommentare lassen sich an einzelne Elemente setzen. Das Modul **Whiteboards** listet die Whiteboards in Verwendung und die Zeichnungen auf, die man öffnen kann; **Neues Whiteboard** erstellt eines.

Solange Personen an einem Whiteboard arbeiten, lässt sich die Zeichnung nicht allein bearbeiten; die Seite bietet stattdessen den Beitritt an.

## Karten der Zusammenhänge

### Sternkarte eines Raums

Die **Sternkarte** zeichnet die Einträge, Bilder und gemeinsamen Labels eines Raums als Karte: Kreise für Einträge (nach Typ eingefärbt), Quadrate für Bilder, Rauten für Labels, gestrichelte Ränder für Einträge anderer Räume. Linien zeigen Seitenbaum, Beziehungen, Erwähnungen, Einbettungen und Kopien. Ziehen verschiebt die Karte, ein gezogener Knoten nimmt seine Nachbarn mit; das Mausrad zoomt. Wählt man einen Knoten, erscheinen seine Angaben. Ein Doppelklick auf einen Knoten (oder Umschalt+Enter) zeigt nur ihn mit seiner Nachbarschaft bis zu einer gewählten **Tiefe** (**Vertiefen**); Escape geht einen Schritt zurück.

Mit **Alle Räume** in der Raumauswahl zeigt die Karte die Einträge aller Räume, die man lesen darf, mit Baum, Beziehungen und gemeinsamen Labels. Auch ihre Fotos und Zeichnungen stehen darauf, jedes mit der Seite oder Aufgabe verbunden, die es enthält, eine Kopie mit ihrem Original; zuerst kommen jene, die ein Text zeigt, damit ein grosses Album sie nicht verdrängt. Sind es mehr, als eine Karte zeigt, steht jede Blase für die Einträge eines Typs in einem Raum, so gross wie ihre Anzahl, und Linien zählen die Verbindungen zwischen ihnen; **Karte des Raums öffnen** führt von einer Blase zur Karte ihres Raums. **Label in allen Räumen zeigen** grenzt die Karte auf die Einträge ein, die ein Label tragen, in allen Räumen; vorgeschlagen werden die verwendeten Labels.

### Auswirkungen

Aus den Angaben eines Knotens zeigt **Auswirkungen**, was von einem Eintrag oder Bild abhängt (**Betroffen**) oder wovon er abhängt (**Abhängig von**), bis zu einer gewählten Tiefe. Die Liste ordnet die erreichten Einträge nach Abstand. Das ist vor dem Ändern oder Löschen von etwas Zentralem nützlich.

### Sternenkarte der Labels

Der Reiter **Labels** zeigt jedes Label als Stern: Je grösser der Stern, desto mehr Einträge tragen es; Linien verbinden Labels, die gemeinsam vorkommen. Man filtert nach Labels, mit oder ohne verbundene Labels, und nach einer Mindestanzahl. **Als Liste** zeigt dasselbe als Tabelle.

## Inhalte schützen

### Einen Eintrag einschränken

Unter **Zugriff** wählt, wer einen Eintrag bearbeiten darf, **Für Personen oder Gruppen schützen**: für **Lesen und Bearbeiten** oder für **Nur Bearbeiten** (alle im Raum lesen weiterhin). **Nur ich darf lesen und bearbeiten** ist der kurze Weg. Eine Einschränkung gilt auch für die Unterseiten und gewährt nie mehr als die Raumrolle. **Einschränkung aufheben** öffnet den Eintrag wieder für den Raum.

### Textstellen und Blöcke schützen

Einzelne Wörter, Absätze oder ganze Blöcke (eine Überschrift mit ihrem Abschnitt, ein Bild, eine Tabelle) lassen sich schützen: beim Bearbeiten markieren, **Schützen** wählen und die Personen und Gruppen nennen, die sie lesen dürfen. Alle anderen sehen einen Platzhalter, der nennt, wer entscheidet. Eine geschützte Stelle lässt sich für weitere Personen freigeben (**Leser ändern**), entschützen oder endgültig löschen.

### Freigabe beantragen

Ein Klick auf eine verborgene Stelle oder einen verborgenen Block erklärt sie und bietet **Freigabe beantragen** an. Der Antrag wird zur Aufgabe für die verantwortliche Person der Seite oder für die Person, die den Inhalt geschützt hat; er erscheint in deren Aufgabenliste. **Freigeben** oder **Ablehnen** beantwortet ihn, und die antragstellende Person wird benachrichtigt.

### Schützenswerte Daten und Datenschutz

Speichert jemand ein Passwort, einen Schlüssel oder eine Kontonummer, schränkt Orbithek den Eintrag sofort auf die speichernde Person ein und teilt ihr dies mit. Hat die Administration Datenschutz-Regelwerke aktiviert, werden Inhalte, die auf besonders schützenswerte Personendaten hinweisen, als Entwurf gehalten, den nur Personen mit Bearbeitungsrecht lesen, bis eine zweite Person sie freigibt.

## Exportieren und drucken

**Exportieren** auf einer Seite bietet:

- **PDF (drucken)** und **Word (.docx)**, jeweils auch **mit Unterseiten (Handbuch)**;
- **Markdown mit Bildern (.zip)**, auch mit Unterseiten;
- jede Tabelle des Textes als **CSV**.

Hat der Typ der Seite eine Word-Vorlage, verwendet der Word-Export sie. Listen von Einträgen und Abfrageergebnisse lassen sich als CSV exportieren.

## Prozess-Engines

Hat die Administration eine Prozess-Engine angebunden, kann ein Text deren Prozesse als BPMN-Diagramme zeigen, mit einer Operations-Sicht der Zählungen, sowie ihre Formulare, Entscheidungen und Architekturskizzen und Startformulare, die beim Absenden einen Prozess starten.

Tasks einer Engine können als verknüpfte Aufgaben in einem Raum erscheinen. Unter **Konto und Passwort → Verbundene Engines** verbindet man das eigene Konto mit einer Engine; danach übernimmt man einen Task, gibt ihn zurück und schliesst ihn mit seinem Formular direkt in Orbithek ab (**Übernehmen**, **Zurückgeben**, **Abschliessen…**). Titel und Felder einer verknüpften Aufgabe gehören der Engine; sie endet, wenn die Engine sie beendet.

## KI-Assistenten

**KI-Zugang** erklärt, wie ein KI-Assistent über das Model Context Protocol (MCP) Seiten liest, durchsucht und schreibt, stets mit genau den eigenen Rechten:

- Für Assistenten auf dem eigenen Rechner erstellt man ein Token (**Lesen und Schreiben** oder **Nur lesen**) und kopiert die vorbereiteten Einstellungen für den eigenen Client.
- Gehostete Assistenten wie Claude registriert die Administration einmal; danach genehmigt jede Person den Assistenten bei der ersten Nutzung und kann die Genehmigung später widerrufen.

Verschieben, Löschen, Einschränkungen und Administration stehen Assistenten bewusst nicht zur Verfügung.

## Das eigene Konto

**Konto und Passwort** im Kontomenü enthält:

- das **Profilbild**, das neben dem Namen erscheint;
- die **Sprache** der Oberfläche;
- die **Darstellung**: wie das System, hell, dunkel oder *Cockpit*;
- **Passwort ändern**;
- **API-Token** für Skripte und Werkzeuge, jeweils mit Ablaufdatum; ein Token wird nur einmal angezeigt und lässt sich widerrufen;
- **Nachrichtenkanäle**: eine E-Mail-Adresse und Webhooks für Benachrichtigungen;
- **Verbundene Engines**.

## Governance

Das Modul **Governance** zeigt für jeden Raum, in dem man eine Rolle hat, die Befunde, die Aufmerksamkeit brauchen:

- **Schützenswerte Daten**: Einträge, die Passwörter, Schlüssel oder Kontonummern zu enthalten scheinen;
- **Unverändert** seit einem Jahr (oder einer gewählten Anzahl Tage);
- **Unplausibel**: formale Widersprüche, etwa ein Ende vor dem Beginn;
- **Personendaten**, die besonders schützenswert sind, sofern Datenschutz-Regelwerke aktiv sind;
- **Wiederprüfung**: Einträge, deren Typ eine periodische Prüfung verlangt; **Als geprüft markieren** bestätigt sie.

Wer einen Eintrag zuletzt geändert hat, gilt als verantwortlich und sieht die eigenen offenen Befunde in den Benachrichtigungen.

## Administration

Die Administration findet unter **Administration**:

- **Kontoanträge**, **Benutzer und Systemrollen**, **Gruppen** und **Raumrolle vergeben**;
- **Räume**: Räume erstellen, ihre Einstellungen öffnen, einen Raum in den Papierkorb legen und wiederherstellen oder endgültig löschen;
- **Typen**: Typen in einem Formular anlegen und ändern — Name, Text, Felder mit ihrer Art und ihren Optionen, Workflow mit Status und Übergängen (oder einfach *offen → erledigt*) und Starttext — für die ganze Installation oder für einen Raum; der Typ als JSON bleibt unter **Erweitert**. Typen entstehen auch aus den Vorlagen;
- **Backup** (auch zeitgesteuert in ein Verzeichnis, per WebDAV, S3, FTPS oder SFTP) und **Speicher**;
- **Kalender anderer Dienste**, **Album: ältere Versionen**, **Fileshares**, **Konnektoren** und **Prozess-Engines**.
  Unter **Fileshares** wird ein Album-Ordner an ein vom Betrieb freigegebenes Verzeichnis oder an einen S3-Bucket gebunden und sofort (**Jetzt lesen**, **Probelauf**) oder nach Zeitplan gelesen.
  Wer den Ordner lesen darf, sieht jedes Bild, das der Server auf dem Share lesen kann; ein Lauf, der mehr als die Hälfte der Bilder eines Shares zurückziehen würde, zieht keines zurück bis **Rückzug bestätigen**.
  **Originale über signierte Links öffnen** lässt die Leserinnen und Leser des Ordners eines S3-Shares dessen Originale über Links öffnen, die der Server für 15 Minuten signiert;
- **Erscheinungsbild (Corporate Design)**: Logo, Farben, Hintergrund, Schriften und ein fertiges Theme, darunter *Capacity*, ein ruhiges Erscheinungsbild für Notizen in Hell und Dunkel;
- **Word-Vorlagen**, **Datenschutz-Regelwerke**, **Import** und das **Audit-Protokoll**.

Die Raumadministration verwaltet Mitglieder und Rollen ihres Raums unter den **Einstellungen** des Raums. Dort legt sie unter **Eintragstypen dieses Raums** mit demselben Formular auch die eigenen Typen des Raums an, ändert und entfernt sie oder beginnt mit einer Vorlage. Entfernt wird ein Typ erst, wenn kein Eintrag und kein Kanban ihn nutzt.

## Tastenkürzel

| Tasten | Aktion |
| --- | --- |
| Ctrl+B, Ctrl+I, Ctrl+U | Fett, kursiv, unterstreichen |
| Ctrl+Shift+X, Ctrl+Shift+H | Durchstreichen, hervorheben |
| Ctrl+Shift+L, E, R, J | Linksbündig, zentriert, rechtsbündig, Blocksatz |
| Ctrl+K | Link einfügen oder bearbeiten; ausserhalb eines Texts die Befehlspalette öffnen |
| Ctrl+Shift+P | Befehlspalette öffnen |
| Alt+Shift+D | Tagesnotiz von heute öffnen |
| Shift+Klick | Verlinkten Eintrag neben dem aktuellen öffnen; Esc schliesst ihn |
| @ | Person oder Eintrag erwähnen |
| Ctrl+Z, Ctrl+Y | Rückgängig, wiederholen |
| Ctrl+Shift+V | Als reinen Text einfügen |
| Ctrl+Enter | Abfrage ausführen |
| Ctrl+Shift+F, F11 | Lesemodus; Esc beendet ihn |
| Shift+Pfeil | Kalendereintrag verschieben |

Auf einem Mac verwendet man Cmd statt Ctrl.

## Hilfe erhalten

Fehlt eine hier beschriebene Funktion, ist sie möglicherweise auf dem Server ausgeschaltet oder die eigene Rolle erlaubt sie nicht; in diesem Fall wendet man sich an die Raumadministration oder an die Administration der Installation. Die Startseite zeigt die verwendete Version von Orbithek.
