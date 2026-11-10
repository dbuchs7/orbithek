# Orbithek-Benutzerhandbuch

Dieses Handbuch erklärt die Arbeit mit Orbithek: Seiten lesen und schreiben, Aufgaben verfolgen, zusammenarbeiten und Informationen finden. Es beschreibt die Funktionen so, wie die Oberfläche sie zeigt. Einige Funktionen stehen nur zur Verfügung, wenn die Administration sie eingeschaltet hat; dies ist jeweils vermerkt.

> [!NOTE]
> Man öffnet dieses Handbuch jederzeit über das Menü des Profilbilds oben rechts: **Handbuch**. Die Sprachlinks über dem Text wechseln zwischen Deutsch und Englisch.

## Grundbegriffe

### Räume, Einträge und Typen

Alles, was in Orbithek geschrieben wird, ist ein **Eintrag**: eine Wiki-Seite, eine Aufgabe, eine Entscheidung, ein Sitzungsprotokoll. Woraus ein Eintrag besteht, legt sein **Typ** fest. Ein Typ bestimmt die Felder des Eintrags (zum Beispiel *assignee*, *due* oder *priority*), ob er einen Text hat und ob er einen **Workflow** mit Status wie *open* und *done* durchläuft. Seiten und Aufgaben sind deshalb keine getrennten Systeme, sondern Einträge unterschiedlicher Typen.

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

- **Neun Punkte** (Module): öffnet den Modulwechsler mit Räume, Erstellen, Aufgaben, Kommentare, Album, Whiteboards, Sternkarte, KI-Zugang, Governance, Konto und, für die Administration, Administration.
- **Startseite**, **Zurück** und **Vorwärts**.
- Das **Logo**, das ebenfalls zur Startseite führt.
- Das **Suchfeld**.
- **+ Neu**: erstellt eine Seite oder einen Eintrag am aktuellen Ort.
- Die **Glocke**: die Benachrichtigungen, mit der Anzahl ungelesener.
- **DE** und **EN**: die Sprache der Oberfläche.
- Den Umschalter für **hellen und dunklen Modus**.
- Den **Lesemodus** (Ctrl+Shift+F, F11): Die Seite füllt den Bildschirm; Esc beendet ihn.
- Das **Profilbild** oder die Initialen: öffnet das Kontomenü mit *Konto und Passwort*, *KI-Zugang*, *Handbuch*, *Administration* (nur für die Administration) und *Abmelden*.

### Die Startseite

Die Startseite begrüsst, zeigt die Anzahl Seiten, Aufgaben und Bilder und bietet einen Schnellzugriff auf die eigenen Aufgaben, Benachrichtigungen, persönlichen Notizen, die Sternkarte, das Album, die Whiteboards und den KI-Zugang. Darunter stehen die eigenen Räume, als Kacheln oder als Liste; ein Stern markiert einen Raum als Favorit. Gibt es noch keine Räume, findet die Administration dort die ersten Schritte: einen Raum erstellen, mit den Vorlagen Typen hinzufügen (Seite, Aufgabe, Entscheidung) und Personen eine Rolle geben.

## Seiten und Einträge

### Eine Seite oder einen Eintrag erstellen

Man wählt **+ Neu** in der oberen Leiste oder **Neue Seite** in einem Raum. Die Erstellungsansicht fragt nach Raum, Typ und Titel und legt den neuen Eintrag unterhalb der Seite an, von der man kam. Auf einer Seite erstellt **Neue Unterseite** eine Seite darunter. Hat ein Typ einen Starttext, beginnt ein neuer Eintrag damit.

### Einen Eintrag lesen

Ein Eintrag zeigt Titel, Schlüssel, Typ und Status sowie seinen Text. Die Seitenleiste **Zu diesem Eintrag** enthält die Angaben:

- **Eigner und Details**: die Felder des Eintrags, bearbeitbar für alle mit Bearbeitungsrecht.
- **Status**: der aktuelle Status und die möglichen Übergänge (**Status ändern**).
- **Labels**, **Beziehungen** und **Dateien**.
- **Zugriff**: wer lesen und bearbeiten darf.
- **Verlauf**: jede Änderung des Eintrags.

Über dem Text stehen **Exportieren** und **Beobachten** zur Verfügung; die Kommentare folgen unter dem Text.

### Einen Text bearbeiten

Mit **Bearbeiten** wird der Text bearbeitet, mit **Fertig** abgeschlossen. Änderungen werden laufend gespeichert; die Statuszeile zeigt *Wird gespeichert…* und danach *Alle Änderungen gespeichert.* Die Werkzeugleiste bietet:

- die Blockart: Absatz, Überschrift 1 bis 3, Aufzählungspunkt, nummerierter Punkt, Aufgabe, Zitat, Codeblock sowie Info-, Hinweis-, Warn-, Tipp- und Fehlerbox;
- Fett, Kursiv, Unterstreichen, Durchstreichen, Hervorheben, Code im Text und Ausrichtung;
- **Link** (Ctrl+K): eine Webadresse oder eine Seite bzw. Aufgabe, gesucht nach ihrem Titel;
- **Erwähnen** (@): eine Person oder einen Eintrag;
- **Rückgängig** und **Wiederholen**.

Eingefügtes Markdown wird zu formatiertem Text. Ctrl+Shift+V fügt stattdessen reinen Text ein.

### Inhalte einfügen

Das Menü **Einfügen…** fügt Blöcke nach dem Cursor ein:

- **Tabelle**, **Trennlinie**, **Inhaltsverzeichnis**, **Bildverzeichnis**, **Tabellenverzeichnis** und **Unterseiten**;
- **Bild**, **Video**, **Zeichnung**, ein Bild **Aus dem Album** oder ein **Kopiertes Bild**;
- **Eintrag einbetten…**: zeigt einen anderen Eintrag vollständig im Text;
- **Neue Aufgabe…** (wird unter der Seite angelegt und als Link oder vollständig gezeigt) und **Aufgabenliste…** (alle Aufgaben mit einem Label oder das Ergebnis einer Abfrage);
- **Markdown einfügen…** und **Markdown-Datei aus einem Repository…** (sofern eingeschaltet);
- BPMN-Diagramme sowie Prozesse, Formulare, Entscheidungen, Startformulare und Architekturskizzen einer Prozess-Engine (siehe *Prozess-Engines*).

Bilder lassen sich links oder rechts mit Text daneben oder zentriert platzieren und durch Ziehen in der Grösse ändern; eine Bildunterschrift nummeriert sie im Bildverzeichnis.

### Labels, Beziehungen und Dateien

**Labels** sind kurze Begriffe, die Einträge über Räume hinweg gruppieren, etwa `release-2` oder `security`. Man fügt sie in der Seitenleiste hinzu; ein Klick auf ein Label listet alle Einträge damit auf und bietet es **Als Board** an.

**Beziehungen** verknüpfen einen Eintrag mit einem anderen unter einem benannten Typ, zum Beispiel *blocks* oder *depends on*: Beziehung und Schlüssel des anderen Eintrags eingeben und **Verknüpfen** wählen. Links und Erwähnungen im Text erscheinen ebenfalls als Beziehungen.

**Dateien** werden mit **Dateien hochladen** angehängt. Wer den Eintrag bearbeiten darf, benennt eine Datei in der Liste mit **Umbenennen** um: Alle Versionen erhalten den neuen Namen, der Dateityp bleibt. Bilder und Zeichnungen aller Einträge erscheinen zudem im Album.

### Verschieben, kopieren und löschen

- **Verschieben …** setzt eine Seite mit ihren Unterseiten unter eine andere Seite oder auf die oberste Ebene des Raums. Innerhalb eines Raums ändern **Nach oben** und **Nach unten** die Reihenfolge.
- **Kopieren …** kopiert eine Seite, auf Wunsch **Mit Unterseiten**.
- **In den Papierkorb** entfernt einen Eintrag aus der Ansicht. Das ist nur möglich, wenn kein anderer Eintrag ausserhalb seines Teilbaums auf ihn verweist. Aus dem Papierkorb lässt er sich mit **Wiederherstellen** zurückholen oder mit **Endgültig löschen** vernichten; das lässt sich nicht rückgängig machen.

### Verlauf und Versionen

Der **Verlauf** listet jede Änderung eines Eintrags mit Person und Zeitpunkt auf. Eine Fassung des Textes zeigt ihre **Änderungen**; **Diese Fassung wiederherstellen** holt einen früheren Text als neue Version zurück. Durch das Wiederherstellen geht nichts verloren.

## Zusammenarbeiten

### Kommentare

Jeder Eintrag hat an seinem Ende Kommentare. Man schreibt einen Kommentar, erwähnt Personen mit @Name und nutzt **Antworten** und **Erledigt**; erledigte Diskussionen bleiben unter *Erledigt* sichtbar und lassen sich wieder öffnen. Für einen Kommentar zu einer Textstelle markiert man Text innerhalb eines Blocks und wählt **Kommentieren**; der Kommentar zeigt die Stelle, und **Diesen Text auf der Seite zeigen** führt zu ihr zurück.

Das Modul **Kommentare** sammelt Kommentare zu beobachteten Einträgen, die eigenen und jene mit Erwähnungen (*Alle*, *Abonniert*, *Meine*, *Erwähnungen*).

### Beobachten und Benachrichtigungen

**Beobachten** auf einer Seite oder in einem Raum meldet, was dort geschieht: neue Unterseiten, Bearbeitungen, Änderungen von Status und Feldern, Kommentare, Labels und Dateien. Die Glocke in der oberen Leiste führt zu den **Benachrichtigungen**, filterbar nach *Erwähnungen*, *Kommentare* und *Änderungen* oder *Nur ungelesene*. Gemeldet wird auch, wenn jemand einen erwähnt, eine Freigabe-Anfrage beantwortet oder eine selbst gespeicherte Seite wegen schützenswerter Daten eingeschränkt wurde. Unter **Konto und Passwort** legt man die Nachrichtenkanäle fest, etwa eine E-Mail-Adresse.

### Persönliche Notizen

Beim Lesen markiert man eine Stelle und wählt **Notiz**, um eine Notiz dazu anzulegen, die nur man selbst sieht. **Meine Notizen** auf der Startseite listet alle eigenen Notizen mit den zugehörigen Stellen auf.

### Gleichzeitig bearbeiten

Mehrere Personen können einen Text gleichzeitig bearbeiten. Jeder Block, an dem jemand arbeitet, ist für die anderen gesperrt; die Zeile über dem Text nennt, wer gerade im Text ist und wer welchen Block bearbeitet. Bricht die Verbindung ab, stellt der Editor sie selbständig wieder her.

## Informationen finden

### Suche

Man gibt Wörter aus Titel, Text oder Kommentar oder einen Schlüssel wie `PLAT-12` in das Suchfeld ein. Während der Eingabe erscheinen Vorschläge; **Enter** zeigt alle Treffer, geordnet nach Relevanz (ein bis fünf Sterne). Die Suche findet nur, was man lesen darf.

### Abfragen

Eine **Abfrage** wählt Einträge nach ihren Feldern aus. Man öffnet sie aus den Suchergebnissen (**Stattdessen die Felder der Einträge abfragen**). Bedingungen werden mit AND, OR und NOT verbunden; Klammern fassen sie zusammen:

- Vergleiche mit `=`, `!=`, `<`, `<=`, `>`, `>=`; `~` findet jedes Wort eines Werts; `IN (a, b)` ist einer aus einer Liste.
- `IS EMPTY` und `IS NOT EMPTY` finden Einträge ohne bzw. mit Wert.
- Ein Datum schreibt man `2026-10-01`; `-7d`, `2w`, `+12h` und `-1y` zählen ab jetzt; `now()`, `startOfWeek()` und ähnliche Funktionen stehen zur Verfügung.
- `currentUser()` steht für die eigene Person.
- `ORDER BY` sortiert, `ASC` oder `DESC`.

Beispiele:

```
type = task AND assignee = currentUser() AND finished = false ORDER BY due
updated >= -7d ORDER BY updated DESC
text ~ "release" AND label IN (web, security)
```

Ctrl+Enter führt eine Abfrage aus. Die Ergebnisse lassen sich als CSV exportieren.

### Ansichten eines Raums

Ein Raum bietet mehrere Ansichten: seinen Seitenbaum, **Alle Einträge**, ein **Board** je Typ mit Workflow, den **Kalender**, die **Roadmap** und, je nach Rolle, **Governance** und **Linkprüfung**. **Beobachten** in einem Raum meldet alles, was darin geschieht.

## Aufgaben

### Die Aufgabenliste

Das Modul **Aufgaben** listet die offene Arbeit in den eigenen Räumen auf: Einträge vom Typ *task* sowie jedes Typs mit Workflow und einem Feld *assignee* oder *due*. Die Reiter zeigen **Mir zugewiesen**, **Nicht zugewiesen** und **Alle offenen**; Filter schränken nach Raum, Status, Person, Fälligkeit und Labels ein. **Übernehmen** weist eine Aufgabe sich selbst zu, **Zurückgeben** gibt sie frei. Überfällige Aufgaben sind markiert.

### Boards und Kanbans

Ein **Board** zeigt die Einträge eines Typs in einem Raum als Spalten ihrer Workflow-Status. Das Ziehen einer Karte in eine andere Spalte führt den Übergang des Workflows aus.

Ein **Kanban** fasst die Aufgaben eines Themas zusammen: Unter *Aufgaben → Kanbans* wählt man **Neues Kanban**, gibt einen Namen, den Typ der Karten und optional ein Label an. Jede Karte darauf trägt das Label. In den **Board-Einstellungen** wählt man:

- die **Spalten**: Status-Spalten führen eine Karte durch den Workflow, eigene Spalten behalten ihren Status;
- eine **Höchstzahl** Karten je Spalte, die eine überfüllte Spalte markiert;
- die Felder **Auf den Karten angezeigt**, die **Farbe der Karten** nach einem Auswahlfeld und **Swimlanes** nach einem Personen- oder Auswahlfeld.

Karten werden mit **Neue Karte** hinzugefügt, für ihre Details geöffnet und mit **Vom Kanban nehmen** entfernt. **Boards nach Label** zeigt die Aufgaben jedes Labels als Board, ohne ein Kanban anzulegen.

### Roadmap und Gantt-Diagramm

Die **Roadmap** zeigt Einträge mit Datumsfeldern auf einer Zeitachse: als **Roadmap nach Label** in Bahnen oder als **Gantt-Diagramm** mit einer Zeile je Eintrag. Man wählt Raum, Typen, die Felder **Beginn** und **Ende**, die Labels und den **Massstab** (Wochen, Monate, Quartale). Einträge, die auf unerledigte Einträge warten, sind markiert.

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
- **Karte**: die Aufnahmeorte.

Man sucht nach Name, Label, Schlüssel oder Titel und schränkt nach Art, Raum und Label ein.

### Hochladen und importieren

Bilder werden dort hinzugefügt, wo man gerade ist, in der Ansicht **Ordner**:

- *In einem Ordner* wählt man **Bilder hochladen**, legt Bilder und ZIP-Archive auf dem Album ab oder fügt ein Bild mit Strg+V ein, etwa ein Bildschirmfoto. Ein leerer Ordner zeigt dafür eine grosse Fläche. **Aus dem Album hinzufügen…** kopiert Bilder, die schon im Album sind, in den Ordner; das Original bleibt, wo es ist.
- *In einer Sammlung* gelten dieselben Wege. Eine Sammlung hält keine eigenen Dateien; das Album fragt deshalb, in welchem Ordner die neuen Bilder abgelegt werden, wobei der zuletzt gewählte Ordner vorne steht, oder legt einen **Neuer Ordner…** an. Jedes Bild erhält die Labels der Sammlung und erscheint so in ihr. **Aus dem Album hinzufügen…** gibt Bildern, die schon im Album sind, die Labels der Sammlung, ohne sie zu kopieren.
- *Ganz oben* im Album fragt das Hochladen, Ablegen oder Einfügen ebenfalls nach dem Ordner, der sich danach öffnet.

Eine PowerPoint-Präsentation kommt mit den Bildern ihrer Folien (siehe *Eine PowerPoint-Präsentation importieren*). Aus Foto-Apps exportierte Bilder werden im Browser entpackt. Sofern eingeschaltet, kommen Fotos auch **Aus Google Fotos** oder **Aus geteiltem iCloud-Album**. HEIC-Fotos werden nicht übernommen; sie sind vorher als JPEG zu exportieren.

Der **Ort beim Import** eines Ordners bestimmt, was mit dem Aufnahmeort eines Fotos geschieht: **Für alle behalten**, **Nur für mich** oder **Aus der Datei entfernen**. Pro Bild lässt sich der Ort später nur für sich behalten oder allen zeigen.

Nennt ein Bild keinen oder einen falschen Ort, setzt ihn, wer das Bild bearbeiten darf, von Hand als **Ort**: im Betrachter oder für mehrere ausgewählte Bilder auf einmal. Die Eingabe eines Namens wie *Zermatt* schlägt Orte vor, sofern die Administration die Vorschläge nicht ausgeschaltet hat; **Auf der Karte wählen** setzt den Ort mit einem Klick auf die Karte. **Ort entfernen** löscht ihn.

### Eine PowerPoint-Präsentation importieren

Eine Präsentation wird zu einem Ordner von Folien, bereit zum Präsentieren mit ihren Foliennotizen:

1. In PowerPoint **Datei › Exportieren › Dateityp ändern › PNG** (oder JPEG) wählen und **Alle Folien** exportieren. PowerPoint schreibt ein Bild pro Folie, etwa *Folie1.PNG* oder *Slide1.PNG*.
2. Im Album den Ordner öffnen, in den die Präsentation gehört, und **Bilder hochladen** wählen. Die *.pptx*-Datei zusammen mit all diesen Bildern wählen oder zusammen mit einem ZIP-Archiv davon; Ablegen auf dem Ordner funktioniert ebenfalls.

Im offenen Ordner entsteht ein Ordner mit dem Namen der Präsentation und öffnet sich. Jede Folie wird zu einem Bild, benannt nach ihrer Stelle und ihrem Titel, etwa *03 Unsere Ziele.png*, sodass das Album die Reihenfolge der Folien behält; die Notizen der Folie werden zu den Foliennotizen des Bildes. Ausgeblendete Folien werden weggelassen, wie eine Bildschirmpräsentation sie weglässt.

Eine Präsentationsdatei enthält keine Bilder ihrer Folien; deshalb sind die exportierten Bilder nötig. Die Bilder werden den Folien über die Zahlen in ihren Namen zugeordnet. Passen sie nicht, wird nichts übernommen und das Album nennt, wie viele Folien und Bilder es gefunden hat. Nur die Bilder und die Notizen gelangen auf den Server, nicht die Präsentationsdatei. Animationen, Übergänge, Videos und die Kommentare der Präsentation werden nicht übernommen; *.ppt*-Dateien älterer Versionen sind zuerst als *.pptx* zu speichern.

### Mit Bildern arbeiten

Ein Klick öffnet ein Bild mit seinen Angaben: Zeitpunkt und Ort der Aufnahme, Kamera, Belichtung, Labels und der zugehörige Eintrag. Die Karte eines Bildes und ihr Menü bieten:

- **Umbenennen**: gibt dem Bild einen anderen Namen (siehe *Bilder und Dateien umbenennen*);
- **Kopieren**: um es mit Ctrl+V in eine Seite einzufügen; die Kopie kennt ihr Original und meldet, wenn es sich geändert hat;
- **Exportieren**: PNG, JPEG oder bei Zeichnungen SVG;
- **Einfügen** in die gerade bearbeitete Seite, **Löschen** und ältere Versionen.

**Mehrere auswählen** wählt viele Bilder auf einmal aus, um Labels hinzuzufügen, Favoriten zu markieren, sie in einen Ordner zu verschieben, sie mit **Als ZIP herunterladen** herunterzuladen oder sie zu löschen.

In den Ordnern verschiebt man ein Bild, indem man es auf einen Ordner zieht: auf einen Ordner, der unter dem geöffneten aufgeführt ist, oder auf einen übergeordneten Ordner im Pfad oben. Ein ausgewähltes Bild nimmt alle ausgewählten Bilder mit. Wie bei **Verschieben** wird ein Bild, das man nicht ändern darf oder das ein Text zeigt, kopiert und bleibt zusätzlich, wo es war. Ohne Maus wählt man die Bilder aus und nutzt **In Ordner verschieben…**.

### Bilder und Dateien umbenennen

Wer ein Bild bearbeiten darf, benennt es im Album um: das Menü **…** der Karte öffnen und **Umbenennen** wählen. Der Name auf der Karte wird zum Eingabefeld. **Enter** oder ein Klick daneben speichert; **Escape** behält den bisherigen Namen. In der Dateiliste einer Seite tut **Umbenennen** neben **Löschen** dasselbe für jede Art von Datei, auch für Dokumente und Videos.

- Man tippt nur den Namen: Der Dateityp bleibt. Eine Zeichnung bleibt eine Zeichnung, und aus *IMG_0042.JPG* wird mit *See* der Name *See.JPG*.
- Alle Versionen der Datei erhalten den neuen Namen, auch die älteren Versionen einer Zeichnung.
- Kopien des Bildes in anderen Seiten behalten ihren Namen, ebenso die Bildunterschriften in einem Text.
- Wird eine Zeichnung neu gezeichnet, behält die neue Version den Namen.

### Bilder präsentieren

Das Album präsentiert die Bilder, die es gerade zeigt, als Folien: einen Ordner oder eine Sammlung öffnen, nach Wunsch mit der Suche oder einem Label eingrenzen und **N Bilder präsentieren** wählen. Die Folien sind nach Namen geordnet, Zahlen nach ihrem Wert, also *2 Plan* vor *10 Ausblick*; wer die Reihenfolge ändern will, benennt die Bilder um.

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

**Foliennotizen** gehören zu einem Bild: auf seiner Karte im Album **Foliennotizen hinzufügen** oder im Referentenfenster **Notizen bearbeiten**. Wer das Bild bearbeiten darf, schreibt sie; wer das Bild sieht, liest sie. Die Notizen bleiben bei allen Versionen einer Zeichnung erhalten.

Beide Fenster müssen im selben Browser auf demselben Computer laufen. Personen an anderen Orten folgen über Bildschirmfreigabe.

## Zeichnungen und Whiteboards

### Zeichnungen

**Einfügen… → Zeichnung** erstellt eine Zeichnung im Text. Der Zeichnungseditor bietet Paletten für ArchiMate, BPMN, Netzwerk- und IT-Symbole. Eine Zeichnung wird als Bild ihrer Seite gespeichert, mit allen Versionen.

### Live-Whiteboards

Jede Zeichnung lässt sich mit **Als Whiteboard öffnen** öffnen: Bis zu 20 Personen und KI-Agenten zeichnen gleichzeitig darauf, wobei jedes Element gesperrt ist, solange jemand daran arbeitet. Ein Whiteboard bietet farbige Notizen (**Notiz**), eine **Abstimmung** mit einer Anzahl Stimmen pro Person (das Ergebnis wird als Kommentar festgehalten), einen **Timer** und **Präsentieren**, dem die anderen Bild für Bild folgen. Kommentare lassen sich an einzelne Elemente setzen. Das Modul **Whiteboards** listet die Whiteboards in Verwendung und die Zeichnungen auf, die man öffnen kann; **Neues Whiteboard** erstellt eines.

Solange Personen an einem Whiteboard arbeiten, lässt sich die Zeichnung nicht allein bearbeiten; die Seite bietet stattdessen den Beitritt an.

## Karten der Zusammenhänge

### Sternkarte eines Raums

Die **Sternkarte** zeichnet die Einträge, Bilder und gemeinsamen Labels eines Raums als Karte: Kreise für Einträge (nach Typ eingefärbt), Quadrate für Bilder, Rauten für Labels, gestrichelte Ränder für Einträge anderer Räume. Linien zeigen Seitenbaum, Beziehungen, Erwähnungen, Einbettungen und Kopien. Ziehen verschiebt die Karte, ein gezogener Knoten nimmt seine Nachbarn mit; das Mausrad zoomt. Wählt man einen Knoten, erscheinen seine Angaben. Ein Doppelklick auf einen Knoten (oder Umschalt+Enter) zeigt nur ihn mit seiner Nachbarschaft bis zu einer gewählten **Tiefe** (**Vertiefen**); Escape geht einen Schritt zurück.

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
- **Typen**: Typen aus den Vorlagen oder als JSON erstellen, mit Feldern, Workflow, Word-Vorlage und Starttext;
- **Backup** (auch zeitgesteuert in ein Verzeichnis, per WebDAV, S3, FTPS oder SFTP) und **Speicher**;
- **Kalender anderer Dienste**, **Album: ältere Versionen**, **Konnektoren** und **Prozess-Engines**;
- **Erscheinungsbild (Corporate Design)**: Logo, Farben, Hintergrund, Schriften;
- **Word-Vorlagen**, **Datenschutz-Regelwerke**, **Import** und das **Audit-Protokoll**.

Die Raumadministration verwaltet Mitglieder und Rollen ihres Raums unter den **Einstellungen** des Raums.

## Tastenkürzel

| Tasten | Aktion |
| --- | --- |
| Ctrl+B, Ctrl+I, Ctrl+U | Fett, kursiv, unterstreichen |
| Ctrl+Shift+X, Ctrl+Shift+H | Durchstreichen, hervorheben |
| Ctrl+Shift+L, E, R, J | Linksbündig, zentriert, rechtsbündig, Blocksatz |
| Ctrl+K | Link einfügen oder bearbeiten |
| @ | Person oder Eintrag erwähnen |
| Ctrl+Z, Ctrl+Y | Rückgängig, wiederholen |
| Ctrl+Shift+V | Als reinen Text einfügen |
| Ctrl+Enter | Abfrage ausführen |
| Ctrl+Shift+F, F11 | Lesemodus; Esc beendet ihn |
| Shift+Pfeil | Kalendereintrag verschieben |

Auf einem Mac verwendet man Cmd statt Ctrl.

## Hilfe erhalten

Fehlt eine hier beschriebene Funktion, ist sie möglicherweise auf dem Server ausgeschaltet oder die eigene Rolle erlaubt sie nicht; in diesem Fall wendet man sich an die Raumadministration oder an die Administration der Installation. Die Startseite zeigt die verwendete Version von Orbithek.
