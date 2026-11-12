# Orbithek user manual

This manual explains how to work with Orbithek: reading and writing pages, keeping track of tasks, working together, and finding what you need. It describes the functions as the user interface shows them. Some functions are available only when your administrator has switched them on; this is said where it applies.

> [!NOTE]
> You open this manual at any time from the menu of your profile picture at the top right: **Manual**. The language links above the text switch between English and German.

## Basic concepts

### Rooms, items and types

Everything people write in Orbithek is an **item**: a wiki page, a task, a decision, a meeting note. What an item consists of is set by its **type**. A type defines the item's fields (for example *assignee*, *due* or *priority*), whether it has a text, and whether it moves through a **workflow** of states such as *open* and *done*. Pages and tasks are therefore not separate systems, but items of different types. A type belongs to the whole installation or to one room: a room's own types are used only for items in that room.

Items live in **rooms**. A room holds the work of a team, a project or a topic. Every room has a short **key** in capitals, such as `PLAT`; every item in it has a key made of the room's key and a number, such as `PLAT-12`. Pages form a tree: every item has at most one parent page.

### Roles and access

What you may do in a room depends on your **role** there:

| Role | What it allows |
| --- | --- |
| Viewer | Read the room's items, comment, take personal notes. |
| Editor | Additionally create, edit, move and delete items. |
| Room administrator | Additionally grant roles in the room and change its settings. |

Administrators of the installation act as room administrators everywhere and also manage types, users and the server settings. Access can be narrowed further for single pages (see *Protecting content*); it is never widened beyond the room role. Whatever you may not read does not appear anywhere for you, neither in search results nor in lists or counts.

## Getting started

### Signing in

Sign in with your username and password, or with single sign-on if your organisation has set it up. If self-service accounts are enabled, **Request an account** on the sign-in page sends a request that an administrator approves. If you forgot your password, ask for a reset link by mail; the link lets you choose a new password and ends your other sessions.

### The top bar

The bar at the top of every page holds, from left to right:

- **Nine dots** (Modules): opens the module switcher with Rooms, Create, Tasks, Today, Comments, Album, Whiteboards, Starmap, AI access, Governance, Account and, for administrators, Administration.
- **Start page**, **Back** and **Forward**.
- The **logo**, which also leads to the start page.
- The **search** field.
- **+ New**: creates a page or item where you are.
- **Today**: your daily note of today (Alt+Shift+D); see *Daily notes*. On a narrow screen it is in the module switcher.
- The **bell**: your notifications, with the number of unread ones.
- **DE** and **EN**: the language of the interface.
- The **light and dark mode** switch.
- **Reading mode** (Ctrl+Shift+F, F11): the page fills the screen; Esc leaves it.
- Your **profile picture** or initials: opens the account menu with *Account and password*, *AI access*, *Manual*, *Administration* (administrators only) and *Log out*.

### The start page

The start page greets you, shows counts of pages, tasks and pictures, and offers quick access to your tasks, notifications, personal notes, the starmap, the album, the whiteboards and AI access. Below it lists your rooms, as tiles or as a list; a star marks a room as a favourite. If there are no rooms yet, administrators find the first steps there: create a room, add types from the ready-made templates (page, task, decision, daily note), and give people a role.

## Pages and items

### Creating a page or item

Choose **+ New** in the top bar or **New page** in a room. The creation view asks for the room, the type and the title, and places the new item below the page you came from. On a page, **New child page** creates a page below it. If a type has a start text, a new item begins with it. A room's own types are offered only when that room is chosen.

### Reading an item

An item shows its title, its key, type and state, and its text. The panel **About this item** at the side holds its details:

- **Owner and details**: the item's fields, editable by those who may edit.
- **Status**: the current state and the available transitions (**Change the status**).
- **Labels**, **Relations** and **Files**.
- **Access**: who may read and edit.
- **History**: every change of the item.

Above the text, **Export** and **Watch** are available; the comments follow below the text.

The section **Context** at the side lists what belongs to the item without being in it: under **Referred to by** the items that mention it, embed it, link to it or relate to it, each with the sentence the mention or link stands in; under **Sharing labels** the items that carry its labels, most labels in common first; and under **In daily notes** your daily notes that name it. Only items you may read are listed, and each list shows its first entries.

Shift+click on a link to another item, or **Beside** next to an entry of the context, opens that item in a pane beside the current one: you read it there, and edit its text with **Edit** if you may. The address keeps the item beside, so the pair can be shared and comes back with **Back**. **Open on its own** shows it as a page; Esc or the × closes the pane. On a phone, the pane follows below the page.

### Editing a text

Choose **Edit** to edit the text and **Done** to finish. Changes are saved continuously; the status line shows *Saving…* and then *All changes saved.* The toolbar offers:

- the block kind: paragraph, heading 1 to 3, bulleted, numbered and task items, quote, code block, and info, note, warning, tip and error panels;
- bold, italic, underline, strike-through, highlight, inline code and alignment;
- **Link** (Ctrl+K): a web address, or a page or task found by its title;
- **Mention** (@): a person or an item;
- **Undo** and **Redo**.

Markdown that you paste becomes formatted text. Ctrl+Shift+V pastes plain text instead.

If no item has the title you type after @ or as the address of a link, the list offers *Create «title» as …*. The dialog **Create and mention** asks for the type (the room's page type unless you choose another) and the room (the room of the text you edit unless you choose another); a page made in the same room goes below the page you edit, and is protected as that page is; from a daily note, it goes to the top of the room. The new item is mentioned where you typed. Types that require fields are made with **+ New** instead. When you may not create in a room, or the storage is almost full, the dialog says so and nothing is created.

### Inserting content

The menu **Insert…** adds blocks after the cursor:

- **Table**, **Divider**, **Table of contents**, **List of figures**, **List of tables** and **Child pages**;
- **Image**, **Video**, **Drawing**, a picture **From the album** or a **Copied picture**;
- **Embed an item…**: shows another item whole inside the text;
- **New task…** (created below the page and shown as a link or whole) and **Task list…** (every task with a label, or the result of a query);
- **View of items…**: the items of a type as a table, a gallery or a list, grouped by a field (see *Views of items*);
- **Paste Markdown…** and **Markdown file from a repository…** (if enabled);
- BPMN diagrams, and processes, forms, decisions, start forms and architecture sketches of a process engine (see *Process engines*).

Pictures can be placed left or right with text beside them, centred, and resized by dragging; a caption numbers them in the list of figures.

### Tables

A table is edited in place. Click a cell to write in it; text that does not fit wraps within the cell. Tab and Shift+Tab move to the next and the previous cell, Enter to the cell below, and Escape leaves the table.

- *Formatting:* the keys of the text work in a cell (Ctrl+B, Ctrl+I, Ctrl+U and the others), and so do the buttons above the table; **Link** (Ctrl+K) links the selected text to a web address.
- *Choosing cells:* click a cell, then click another with Shift held: the cells between them are chosen. The buttons then format all of each chosen cell.
- *Colours:* **Fill** fills the chosen cells and **Border** colours their borders, each in one of eight tones; **None** takes the colour off. The tones read well in the light and the dark theme.
- *Sizes:* drag the right edge of a cell to make its column wider or narrower, and its lower edge to make its row taller. A table wider than the text scrolls sideways. **Reset sizes** lets the text decide again.
- *Rows and columns:* **+ Row** adds a row below the chosen cell, **+ Column** a column to its right; **− Row** and **− Column** remove the chosen cell's row or column.

Formatting, colours and sizes are kept in Word and PDF exports; a Markdown export keeps the formatting.

### Labels, relations and files

**Labels** are short words that group items across rooms, such as `release-2` or `security`. Add them in the panel at the side; a click on a label lists every item carrying it and offers it **As a board**. A label may have a page that describes it (see *Label pages and collections*).

**Relations** link an item to another with a named type, for example *blocks* or *depends on*: enter the relation and the other item's key and choose **Relate**. Links and mentions in the text are shown as relations too.

**Files** are attached with **Upload files**. Whoever may edit the item renames a file in the list with **Rename**: every version takes the new name, and the file type stays. Pictures and drawings of every item also appear in the album.

### Reading PDF files

A PDF file of an item opens in the reader with **Read** in the file list, or with a click on a link to it in the item's text. The reader shows the pages with their text, which can be selected and copied. The bar above them moves from page to page, zooms in and out or to **Fit width**, searches the document (Enter for the next match) and offers **Download**. At the side, *Pages* shows small pictures of the pages, *Contents* the document's table of contents when it has one, and *Highlights* the highlights made in it. The address names the page, so a link leads others to the same page.

Whoever may comment on the item highlights text: select it on one page, then choose a colour to highlight it at once, or **Highlight with a note**, choose a colour, write the note and **Save highlight**. A highlight is a comment on the item: it appears among the item's comments with its file, page and quote, and **Show in the PDF** leads to it. Highlights are answered, resolved and deleted in the reader's list as in the comments; **Open in the comments** leads to the item. When a newer version of the file is uploaded, the reader shows the newest version and finds each highlight again by its text; a highlight whose text is no longer in the file says so. **Export as Markdown** saves the highlights with their quote, page and note. A PDF with highlights stays in its item: it cannot be moved to another one.
### Fields of an item

The fields of an item stand in the panel at the side and are saved as soon as they change. Besides text, numbers, dates, people and lists, a field can be:

- **Item**: a reference to another item, chosen by typing a few letters of its key or title. It is shown by key and title and counts as a relation: the other item lists it among its relations, the starmap and the impact view show it, and the other item cannot be deleted while it is referred to. An item you may not read is not shown.
- **Several of a list**: any of the field's values, ticked.
- **Rating**: one to five stars.
- **File**: one of the item's own files, such as a cover picture, shown as a small picture. Upload the file first, then choose it.

### Moving, copying and deleting

- **Move…** puts a page with its child pages below another page or at the top of the room. Within a room, **Move up** and **Move down** change the order.
- **Copy…** copies a page, optionally **With its child pages**.
- **Move to trash** removes an item from sight. It is possible only when no other item outside its subtree refers to it. From the trash it can be restored with **Restore**, or erased with **Delete for good**, which cannot be undone.

### History and versions

**History** lists every change of an item, who made it and when. A version of the text shows its **Changes**; **Restore this version** brings an earlier text back as a new version. Nothing is lost by restoring.

### Importing notes

Room administrators bring notes written elsewhere into their room with **Import notes** in the room's views; administrators find the same form under **Administration → Import**. You upload a ZIP archive of one of these:

- a folder of Markdown notes, such as an Obsidian vault zipped as it is, or the Markdown export of any notes application that writes Markdown files with front matter;
- a Notion workspace export in the format *Markdown & CSV*.

What the import makes of it:

- every folder that holds notes becomes a page, every note a page below its folder's page; a note named like its folder, `index.md` or `README.md` is the folder page's own text;
- the title is the front matter's `title`, else the first heading, else the file name; in a Notion export the ID at the end of each name is dropped and identifies the page;
- links between notes — `[[Note]]`, `[[Note|text]]`, `[[Note#Heading]]` and relative Markdown links — become mentions of the imported pages; a note embedded with `![[Note]]` becomes a mention too;
- pictures in the archive (`![[picture.png]]` or a relative picture link) are attached and shown; pictures on the web stay links;
- `tags` in the front matter and inline `#tags` become labels (a tag `a/b` becomes the label `a.b`); a front matter key named like a field of the **Type of the pages** fills that field, a `status` the type's workflow knows sets the state, and everything else is kept in a small table at the top of the page;
- each database of a Notion export (a CSV file) becomes a page that lists its rows, and each row an item of the **Type of the rows of databases**, its columns filling fields by name.

Always start with **Dry run**: it writes nothing and lists how many pages would be made or changed and everything that cannot be converted, such as a link to a note the archive does not hold or an embedded PDF. **Import** then writes the pages.

The **Name of the import** identifies it. Import a newer archive under the same name to bring changes over: nothing is made twice, a page whose note did not change keeps what was written in it here, a changed note replaces its page's text (the history keeps the old one), labels are only added, and a page whose note is no longer in the archive is kept, never deleted.

### Importing highlights

An administrator can set up a connector of the kind Readwise under **Administration → Connectors**. It reads your highlights from Readwise on its schedule, with a dry run first like every connector: each book or article becomes an item of the chosen type, with its title, author, source address and cover, labels from its tags, and its highlights as quotes with their notes and locations. A run reads only what changed since the last one. The highlights stand above a line in the item's text; what you write below that line stays when the highlights are brought up to date.
## Web links and capturing

### Saving a web link

A page of the web can be kept as an item of its own, a *web link*: its address, and what the page says about itself, such as its site, description, author and day of publication, in its fields; its picture as a file; and your own notes in its text, where comments and personal notes work as on any page.

While editing a text, paste an address on its own line. The bubble below the link offers **Save as web link item**: the item goes below the page you are writing, or into another room, and the line with the address becomes a card of the page, with its picture, title, site and description. A web link item shows the same card at its top, and so does every text that embeds it.

If your server reads web pages (an administrator decides), a web link's fields and picture are filled from the page. On the web link item, **Read the page again** brings them up to date, keeping an earlier picture as an earlier version; **Take over the page's text** adds the page's main text below the item's text, to comment on it and annotate it with personal notes. Without this, a web link keeps its address, and you fill in its fields yourself.

### Capturing from anywhere

**Capture** saves a page of the web, a quote or a thought from wherever you are. It opens:

- from **New**, with the card **Capture**;
- from any page of the web, with the bookmark offered under **Account and password** in the account menu: drag the button **Capture** to your browser's bookmarks, or use **Copy the bookmark**. On a page, it opens the capture view with the page's title, its address and the text you selected;
- on a phone, by sharing a page or a text to Orbithek once you installed it as an app from the browser's menu.

The capture view shows the title, the address and the text, which you may change, and asks where to save them: as **a web link** (when there is an address), into **today's daily note** (in the room you chose for your daily notes), as **a new item** of a type and in a room you choose, or as **an addition to an item** that you find by searching. The text becomes quotes, and the address follows as their source. Nothing is saved before you choose **Save**. Signing in first keeps what was shared.

## Working together

### Comments

Every item has comments at its end. Write a comment, mention people with @name, and use **Reply** and **Resolve**; resolved threads stay visible under *Resolved* and can be reopened. To comment on a passage, select text within one block and choose **Comment**; the comment shows the passage, and **Show this text in the page** leads back to it.

The module **Comments** gathers comments on the items you watch, your own and those that mention you (*All*, *Watched*, *Mine*, *Mentioning me*).

### Watching and notifications

**Watch** on a page or a room tells you of what happens there: new child pages, edits, changes of state and fields, comments, labels and files. The bell in the top bar leads to your **Notifications**, filtered by *Mentions*, *Comments* and *Changes*, or *Unread only*. You are also told when someone mentions you, answers an access request or when a page you saved was restricted because of sensitive data. Under **Account and password** you set the channels through which messages reach you, such as a mail address.

### Personal notes

Select a passage while reading and choose **Note** to keep a note on it that only you see. **My notes** on the start page lists all your notes with the passages they belong to.

### Daily notes

A daily note is a page of your own for one day: only you read and edit it. **Today** in the top bar, Alt+Shift+D or the command palette opens today's note; it is made the first time you open it, in the **Room for daily notes** you choose once, among the rooms where you may create pages. Until a room is chosen, the choice stands in the note's place; later you change the room in the column at the side, which its arrow folds away and shows again. Notes already made stay where they are.

The note's text is edited in place. **Previous day** and **Next day** move between days, and the month beside marks the days that have a note; a day without one offers **Start this day's note**. Beside the note, **In the calendar this day** lists the items whose dates fall on that day, and **Created or changed this day** the items you may read that were created that day or last changed on it. **Open as a page** shows the note as an item, with its comments, history and fields. Daily notes carry the label `daily-note`, so a query by that label lists them all.

### Editing at the same time

Several people can edit a text at once. Each block someone is working on is locked for the others; the line above the text names who is in the text right now and who is editing which block. If the connection is lost, the editor reconnects on its own.

## Finding information

### Search

Type words of a title, a text or a comment, or a key such as `PLAT-12`, into the search field. Suggestions appear while you type; **Enter** shows all results, ordered by relevance (one to five stars). The search finds only what you may read.

### Command palette

Ctrl+K (when you are not editing a text, where Ctrl+K inserts a link) or Ctrl+Shift+P opens the **Command palette**. Type part of a title, a key or a command: it finds items, offers your **Recently opened** items (remembered in this browser) and **Commands** such as a new item, today's daily note, a module or room, the query, the calendar, the light or dark look, reading mode and the manual. The letters you type need only appear in order, so `tdn` finds *Today's daily note*. Arrow keys choose, Enter opens, Esc closes.

### Queries

A **query** selects items by their fields. Open it from the search results (**Query the fields of the items instead**). Conditions are joined with AND, OR and NOT; parentheses group them:

- Compare with `=`, `!=`, `<`, `<=`, `>`, `>=`; `~` finds every word of a value; `IN (a, b)` is one of a list.
- `IS EMPTY` and `IS NOT EMPTY` find items without or with a value.
- Dates are written `2026-10-01`; `-7d`, `2w`, `+12h` and `-1y` count from now; `now()`, `startOfWeek()` and similar functions are available.
- `currentUser()` stands for you.
- An item field takes an item's key, such as `epic = PLAT-3`; `platforms = web` finds items that have *web* among several values; a rating compares as a number, such as `rating >= 4`.
- `ORDER BY` sorts, `ASC` or `DESC`.

Examples:

```
type = task AND assignee = currentUser() AND finished = false ORDER BY due
updated >= -7d ORDER BY updated DESC
text ~ "release" AND label IN (web, security)
```

Ctrl+Enter runs a query. The results can be exported as CSV.

### Views of a room

A room offers several views: its page tree, **All items**, a **Board** per type with a workflow, **Views** of its items, the **Calendar**, the **Roadmap** and, depending on your role, **Governance** and **Link check**. **Watch** on a room tells you of everything that happens in it.

### Views of items

**Views** in a room, or **View of a type…** beside it, shows the items you may read as a **Table**, a **Gallery** of cards or a **List** of compact rows. Choose the **Room** (or all rooms), the **Type** and, if you like, a **Query** in the query language, then **Apply**. **Group by** puts the items under headers with their counts: by state, by label, or by a field that holds a choice, a person or yes and no; a click on a header folds the group. **Sort** orders them by a column, as does a click on a column's heading, a second click the other way. **Columns** chooses what the table shows; the cards of a gallery show the first picture of the text, or a file field's file, with the title and the chosen fields.

Where you may edit an item, the pencil beside a cell changes it in place: the title, a field, or the state, by one of the workflow's transitions. The address keeps the whole view, so a link shows it to others as far as they may read the items.

In a text, **Insert…** → **View of items…** places such a view; **Change the view…** changes it while editing, and **Open as a view** shows it on a page of its own. Every reader sees only the items they may read.

### Label pages and collections

A label can have a page that says what it is about. On a label's list of items, **Describe this label** makes that page, of the type *topic*, in a room you choose; afterwards the label's list shows it on top with **Read the page**. If several rooms have a page for the same label, you see the oldest one you may read.

A label's page shows the items carrying its label as a view: **Table**, **Gallery** or **List**, grouped and sorted as you choose. With **Order set by hand**, whoever may edit the page drags an item before another or moves it with the arrows (Alt+↑ and Alt+↓); items added later come after the ordered ones, and items you may not read keep their places without being shown. **Forget the order set by hand** lists them in the order they were made again. Such a page with its order is a collection: **Add to collection…** on an item gives it the label of a page you choose.

## Tasks

### The task list

The module **Tasks** lists open work across your rooms: items of the type *task* and of every type with a workflow and an *assignee* or *due* field. The tabs show **Assigned to me**, **Unassigned** and **All open**; filters narrow by room, state, person, due date and labels. **Take over** assigns a task to you, **Give back** releases it. Overdue tasks are marked.

The ready-made type *task* has a *start* date beside its *due* date. The calendar and the roadmap show a task with both as a bar from its start to its due date. A start date is not a deadline: on a kanban a past start date is not marked as overdue. Types created before gain the field when an administrator adds the ready-made types' new fields on the start page.

### Boards and kanbans

A **board** shows the items of one type in a room as columns of their workflow states. Dragging a card to another column carries out the workflow's transition.

A **kanban** gathers the tasks of a topic: choose **New kanban** under *Tasks → Kanbans*, give it a name, the type of its cards and optionally a label. Every card on it carries the label. In the **Board settings** you choose:

- the **Columns**: state columns move a card through the workflow; own columns keep its state;
- a **Limit** of cards per column, which marks an overfull column;
- the fields **Shown on the cards**, the **Colour of the cards** by a choice field, and **Swimlanes** by a person or choice field.

Cards are added with **New card**, opened for their details, and taken off with **Take off this kanban**. **Boards by label** shows every label's tasks as a board without creating a kanban.

### Roadmap and Gantt chart

**Roadmap** shows items with date fields on a time axis: as a **Roadmap by label** in lanes, or as a **Gantt chart** with one row per item. Choose the room, the types, the **Start** and **End** fields, the labels and the **Scale** (weeks, months, quarters). Items that wait on unfinished items are marked.

**Export** saves the chart as a picture, as **PNG** or **JPEG**, with every row and the whole time axis, also what is scrolled out of sight. The picture shows what you see, in the current look, and is named after the view and the day, such as *Gantt chart PLAT 2026-11-12.png*. A chart too large for a picture the browser can draw is refused with a note; narrow it by labels, type or room, or choose a coarser scale.

### Calendar

The **Calendar** shows items with date fields as a **Month**, **Week** or **Agenda**. **Filter** chooses the room, the type, who is assigned, the status, words of the text and labels; a sentence above the calendar says what is shown. Under **Advanced** you choose the date fields and an additional query.

- Where you may edit, drag an entry to another day, or move it with Shift and an arrow key. In the week view, a meeting dropped at another hour keeps its length.
- A repeating entry asks whether to move **Only this occurrence** or **The whole series**; Delete cancels one occurrence.
- Where you may edit a room, double-click a day, or an hour in the week view, to make a new entry there; **New entry** does the same for today. The dialog asks for the type, the room, a title and the fields the type requires, and fills in the date from the place clicked. **Create** keeps you in the calendar with the new entry in focus; **Create and open** opens it.
- **Save view…** keeps a view under a name. If calendar feeds are enabled, **Subscribe (iCal)** gives an address for a calendar program. Whoever knows this address sees the entries without signing in; **New address** and **End feed** withdraw it.
- If your administrator allows it, **Other calendars** shows your calendars of other services beside Orbithek's: iCalendar addresses, Google accounts and CalDAV accounts such as Apple iCloud. They are shown read-only and only to you.

## Album and pictures

### Views of the album

The **Album** shows the pictures and drawings of every page and task you may read. Choose how it shows them:

- **Folders**: pictures in folders of a room; a **collection** gathers labelled pictures of every room;
- **Timeline**: by the month they were taken;
- **Favourites**: the pictures you marked with the star;
- **On this day**: pictures taken on this day in earlier years;
- **Map**: the places where pictures were taken;
- **Duplicates**: pictures that are the same file or look the same (see *Duplicates*).

Search by name, label, key or title, and narrow by kind, room and label. The search also takes a date and finds the pictures taken then, or added then when a picture tells no time: a year (*2026*), a month (*11.2026*, *2026-11*, *November 2026*), a day (*10.11.2026*, *2026-11-10*, *10 November 2026*), or a day or month without a year (*10.11.*, *November*), which finds it in every year.

In the **Folders** view, the path to the open folder stands above its contents, with what changes the folder itself beside it: **New folder**, **New collection** and **Delete folder**, or **Change rule** and **Delete collection** in a collection. Below it are the ways to add pictures, then the folders, and right above the pictures their number, **Choose several** and **Present N pictures**.

### Uploading and importing

Pictures are added where you are, in the **Folders** view:

- *In a folder*, choose **Upload pictures**, drop pictures and ZIP archives onto the album, or paste a picture with Ctrl+V, such as a screenshot. An empty folder shows a large area for this. **Add from the album…** copies pictures that are already in the album into the folder; the original stays where it is.
- *In a collection*, the same ways work. A collection holds no files of its own, so the album asks which folder the new pictures are kept in, the folder chosen last coming first, or makes a **New folder…**; each picture gets the collection's labels and so shows in the collection. **Add from the album…** gives pictures already in the album the collection's labels, without copying them.
- *At the top* of the album, uploading, dropping or pasting asks for the folder as well, and that folder then opens.

A PowerPoint presentation becomes a folder of slides with their notes (see *Importing a PowerPoint presentation*). Pictures exported from photo apps are unpacked in the browser. If enabled, photos also come **From Google Photos** or **From a shared iCloud album**. HEIC photos are not accepted; export them as JPEG first. Videos from Google Photos and shared iCloud albums come in too, with the picture the service keeps of them as their poster.

A folder's **Place on import** decides what happens to the place where a photo was taken: **Keep for every reader**, **Keep to me only** or **Remove from the file**. Per picture, the place can later be kept to you or shown to every reader.

Where a picture tells no place, or a wrong one, whoever may edit it sets the **Place** by hand: in the viewer, or for several chosen pictures at once. Typing a name such as *Zermatt* suggests places, unless an administrator has turned the suggestions off; **Choose on the map** sets the place with a click on the map. **Remove the place** clears it.

Where a picture tells no time, or a wrong one, such as a scan or a photo from a camera whose clock was wrong, whoever may edit it sets the **Time taken** by hand. In the viewer, enter the date and time and, where known, the **Offset from UTC**, then choose **Set the time**; **Back to the file's time** returns to the time the file itself tells. For several chosen pictures, **Set the time…** offers **Set to**, which gives them all one time, and **Shift by**, which moves each by the same days, hours and minutes, later or earlier; pictures that tell no time are not shifted. The album, the timeline and *On this day* then use the time set by hand, and every download, ZIP archive and copy carries it in the picture's EXIF data, while the stored file stays as it was. A video keeps its own time in its file.

### Videos

Videos come into the album the same ways as pictures: uploaded, dropped, pasted, in a ZIP archive or from Google Photos and shared iCloud albums, as MP4, WebM or MOV (iPhone), each up to 2 GB unless an administrator set another limit. The album then handles them like pictures: in folders and collections, on the timeline and the map, as favourites and memories, with labels, renaming, moving, deleting and the ZIP download.

- When a video is uploaded, the browser takes a picture from it, which the album shows on the card with the video's length. A video the browser cannot play keeps a symbol instead.
- When and where it was taken, how long it plays and the camera are read from the video itself; **Place on import** works as for photos.
- A click plays the video in the viewer. Whether a MOV video plays depends on the browser: Safari plays it, other browsers often only when it was recorded as H.264 rather than HEVC. Otherwise choose **Download**.
- The kind filter **Videos** shows only videos. Videos are not exported as PNG or JPEG.
- When presenting, a video is a slide: it shows its poster until the space bar or a click starts it, in the presentation window, with sound there only. The space bar pauses it again; the arrow keys go on. It cannot be zoomed or pointed at.

### Duplicates

Before pictures and videos are uploaded, dropped or pasted, the browser checks whether they are already in the album, in every folder you may read:

- **Same file**: a picture with exactly the same content;
- **Looks the same**: the same picture saved again, smaller, in another format or without its place. Its proportions must be the same; a picture cut differently is no duplicate.

Files chosen twice count too. If duplicates are found, a dialog lists each file with the pictures it matches and their folders. Ticked files are skipped: **Skip ticked files** adds the others, **Add all** adds everything, **Cancel** adds nothing. Videos are compared as files only. Copies made with **Add from the album…** are no duplicates of their original. The check needs a secure connection (HTTPS); without one, files are added unchecked.

The view **Duplicates** shows groups of pictures that are the same file or look the same, newest group first, as far as the room filter narrows it. Choose pictures there and delete them as in every view. **Delete duplicates…** deletes them all at once after a warning: of every set of the same file one picture stays, one a text shows or you may not change, else the largest, else the oldest. Tick the box to also delete pictures that only look the same, so that one picture of each group stays. Deleted pictures cannot be restored. Pictures added before this check existed are compared as the album gets to them: the view says how many still wait and compares more each time it opens.

### Faces and people

When you open a picture you may edit, the browser looks for faces on it; **Find faces** in the viewer looks again. Under **People in the picture** each face has a field for the name: type a name, or choose one of the names given before. **Remove** takes a face away. Everyone who may read the picture sees the names, and a frame on each face.

The view **People** lists everyone named, with their face and how many pictures show them; a click opens that person's album, which can be presented as slides. **Find faces in all pictures** looks for faces on every picture shown that you may edit and that was never searched or, while recognition is on, is not recognised yet. Faces are looked for on uploaded pictures too, after the upload.

Faces are found in your browser; nothing is sent elsewhere. If an administrator turned on **Recognise faces again**, the browser also computes numbers from each face that let the album suggest a name, such as *Ada?*, for a face that resembles faces already named. These numbers are biometric data: turn this on only if the people shown agree. Turning it off forgets them all; the names stay. Pictures searched before recognition was turned on are not recognised until they are searched again: after turning it on, administrators are pointed to **Find faces in all pictures**, and a picture opened in the viewer is searched again on its own.

With recognition on, a person's album also shows **Is this Ada?**: faces on other pictures you may edit that resemble Ada more than anyone else named, most alike first. Click the faces that show Ada, or **Choose all** and click away the wrong ones, then **Confirm chosen**. The faces not chosen stay unnamed and are suggested again later. Only pictures whose faces were searched while recognition was on can be suggested.

### Moving a folder

**Move folder** in an open folder moves it with its folders, pictures and videos into another folder or to the top level of a room. Choose the place under **Move to…** and click **Move**; the path then leads to the folder where it went. A folder can also be dragged onto a folder listed beside it or onto a folder above it in the path; dragged onto **All pictures**, it goes to the top level of its room. Moved into another room, the folder is asked about first: afterwards that room decides who may see and change it. A folder is moved only by someone who may change it and everything in it and may create where it goes; it cannot go into itself or a folder below it.

### Deleting a folder

**Delete folder** in an open folder moves it to the trash with its folders, pictures and videos, after saying how many go with it. Its page restores it with **Restore**. A folder is not deleted while something outside it, such as a page, still uses what is in it; the album names it.

**Delete collection** in an open collection moves only the collection to the trash. Its pictures and videos stay where they are, with their labels; its page restores it with **Restore**.

### Importing a PowerPoint presentation

A presentation becomes a folder of slides, ready to present with its speaker's notes. In the album, open the folder the presentation belongs in and choose **Upload pictures**, then choose the *.pptx* file — or simply drop it onto the folder.

A folder named after the presentation is made in the open folder and opens. Each slide becomes a picture named by its place and title, such as *03 Our goals.png*, so the album keeps the order of the slides, and the slide's notes become the picture's speaker's notes. Hidden slides are left out, as a slide show leaves them out.

A presentation file holds no pictures of its slides, so the browser draws each slide itself: backgrounds, text with its bullets and numbering, shapes, pictures, tables and SmartArt, in the presentation's colours and fonts. The result is close to the original, but not always the same: a font your computer lacks is replaced by a similar one, so lines may break differently, and effects such as shadows are left out. Charts, embedded objects and pictures in formats the browser cannot show (such as EMF) appear as grey boxes; the album then says how many slides hold them.

For exact slides, export them from PowerPoint first: **File › Export › Change File Type › PNG** (or JPEG), **All Slides**. Choose the *.pptx* file together with all these pictures, or with a ZIP archive of them. The album then takes the pictures instead of drawing, matched to the slides by the numbers in their names, such as *Slide1.PNG* or *Folie1.PNG*. If they do not match, nothing is brought in and the album says how many slides and pictures it found.

Only the pictures and the notes reach the server, not the presentation file. Animations, transitions, videos and the presentation's own comments are not taken over; *.ppt* files of older versions must first be saved as *.pptx*.

### Working with pictures

A click opens a picture with its details: when and where it was taken, camera, exposure, labels and the item it belongs to. A picture's card and its menu offer:

- **Rename**: gives the picture another name (see *Renaming pictures and files*);
- **Copy**: to paste it into a page with Ctrl+V; the copy remembers its original and tells when the original changed;
- **Export**: PNG, JPEG, or SVG for drawings;
- **Insert** into the page being edited, **Delete**, and older versions.

**Choose several** selects many pictures at once to add labels, mark favourites, move them to a folder, download them with **Download as ZIP** or delete them all at once with **Delete**; **Choose all shown** chooses every picture shown.

In the folders, a picture is moved by dragging it onto a folder: onto a folder listed below the open one, or onto a folder above it in the path at the top. A chosen picture takes every chosen picture with it. A moved picture stays the same picture: all its versions, labels, speaker's notes, place and favourite marks go with it, and copies of it in pages still find their original. A picture that you may not change, that a text shows or that is protected, and a drawing that comments are on, stay where they are; the album says which and why. Without a mouse, choose the pictures and use **Move to folder…**, which moves them the same way.

### Renaming pictures and files

Whoever may edit a picture renames it in the album: open the card's menu **…** and choose **Rename**. The name on the card becomes a field. **Enter**, or a click elsewhere, saves; **Escape** keeps the old name. In a page's list of files, **Rename** beside **Delete** does the same for every kind of file, documents and videos included.

- You type the name only: the file type stays. A drawing stays a drawing, and *IMG_0042.JPG* renamed to *Lake* becomes *Lake.JPG*.
- Every version of the file takes the new name, the older versions of a drawing included.
- Copies of the picture in other pages keep their names, and so do the captions of pictures in a text.
- When a drawing is drawn again, its new version keeps the name.

### Presenting pictures

The album presents the pictures it shows as slides: open a folder or a collection, narrow it with the search or a label if you like, and choose **Present N pictures**. The slides of a folder or collection follow its order of slides (see *Ordering slides by hand*); pictures not yet placed come after, by name, numbers by their value, so *2 Plan* comes before *10 Outlook*.

The click opens a second window, the **presentation window**, for the projector or a second screen. The album's window becomes the **presenter's window**. It shows the current and the next slide, the speaker's notes, the time since the start and the time of day, and a strip of all slides; a click on a slide in the strip goes there. If your browser blocked the second window, allow pop-ups for this site and choose **Presentation window**. In the presentation window, **Full screen** or **F** fills the screen.

Both windows obey the same keys and buttons:

| What | How |
|---|---|
| Next or previous slide | Arrow keys, Page Up/Down, Space, the buttons; a click in the presentation window moves on |
| First or last slide | Home, End |
| Zoom | + and −, the buttons, or the mouse wheel where it points; drag a zoomed slide to move it; **0** or a double click shows the whole slide |
| Laser pointer | **L** or **Laser**, then point with the mouse; the red dot shows in both windows |
| Black screen | **B** or **Black**, and again to show the slide |

Presentation remotes that send Page Up and Page Down work too. **End** closes the presentation window and returns to the album.

**Speaker's notes** belong to a picture: on its card in the album, **Add speaker's notes**, or in the presenter's window **Edit notes**. Whoever may edit the picture writes them; whoever sees the picture reads them. Notes stay with every version of a drawing.

Both windows must be in the same browser on the same computer. People elsewhere follow by screen sharing.

### Ordering slides by hand

A folder or a collection keeps its own order of slides, the same for everyone who reads it. In an open folder or collection, choose **Order of the slides** instead of **Newest first**: the album shows the pictures in the order they are presented. Whoever may edit the folder or collection drags a picture onto another one: dropped onto its left half it goes before it, onto its right half after it. Chosen pictures move together. Without a mouse, the menu of a picture offers **Move one place earlier** and **Move one place later**.

In the presenter's window, the slides of the strip are dragged the same way; the order is kept at once and the presentation window follows.

Pictures added later come after the placed ones, by name; a new version of a drawing keeps its place. **Order by name** forgets the order set by hand. To place a picture after the last one shown while more follow, choose **Show more** first.

## Drawings and whiteboards

### Drawings

**Insert… → Drawing** creates a drawing in the text. The drawing editor offers palettes for ArchiMate, BPMN, network and IT symbols. A drawing is saved as a picture of its page, with all its versions.

### Live whiteboards

Every drawing can be opened with **Open as whiteboard**: up to 20 people and AI agents draw on it at the same time, each element locked while someone works on it. A whiteboard offers coloured notes (**Note**), a **Vote** with a number of votes per person (the result is written as a comment), a **Timer** and **Present**, which others follow frame by frame. Comments can be placed on single elements. The module **Whiteboards** lists the whiteboards in use and the drawings you can open; **New whiteboard** creates one.

While people work on a whiteboard, the drawing cannot be edited alone; the page offers to join instead.

## Maps of connections

### Starmap of a room

The **Starmap** draws a room's items, pictures and shared labels as a map: circles for items (coloured by type), squares for pictures, diamonds for labels, dashed outlines for items of other rooms. Lines show the page tree, relations, mentions, embeds and copies. Drag the map to move it and a node to pull it with its neighbours; turn the mouse wheel to zoom. Choose a node to see its panel. A double-click on a node (or Shift+Enter) shows only it with its neighbourhood at a chosen **Depth** (**Separate**); Escape steps back.

Choose **All rooms** in the room list to see the items of every room you may read on one map, with their tree, relations and shared labels. Their pictures and drawings are on it too, each joined to the page or task that holds it and a copy to its original; those a text shows come first, so a large album does not crowd them out. When they are more than one map shows, each bubble stands for the items of one type in one room, as big as their number, and lines count the links between them; a bubble opens its room's map with **Open the room's map**. **Show a label in every room** narrows the map to the items that carry a label, in every room; its suggestions are the labels in use.

### Impact

From a node's panel, **Impact** shows what depends on an item or picture (**Affected**) or what it depends on (**Depends on**), up to a chosen depth. The list orders the reached items by distance. Use it before changing or deleting something central.

### Starmap of the labels

The tab **Labels** shows every label as a star: the bigger the star, the more items carry it; lines join labels carried together. Filter by labels, with or without their connected labels, and by a least count. **As a list** shows the same as a table.

## Protecting content

### Restricting an item

Under **Access**, those who may edit an item **Protect for people or groups**: for **Reading and editing**, or for **Editing only** (everyone in the room still reads). **Only I may read and edit** is the short way. A restriction applies to the child pages too and never grants more than the room role. **Lift the restriction** opens the item to the room again.

### Protecting passages and blocks

Single words, paragraphs or whole blocks (a heading with its section, a picture, a table) can be protected: select them while editing and choose **Protect**, then name the people and groups who may read them. Everyone else reads a placeholder that names who decides. A protected passage can be opened to more readers (**Change readers**), unprotected, or erased for good.

### Asking for access

A click on a veiled passage or block explains it and offers **Ask for access**. The request becomes a task for the page's owner, or for the person who protected it; it appears in their task list. **Grant** or **Decline** answers it, and the requester is notified.

### Sensitive data and data protection

When someone saves a password, a key or an account number, Orbithek restricts the item at once to the person who saved it and tells them so. If your administrator has activated data protection regulations, content that points to personal data in need of protection is kept as a draft that only editors read, until a second person releases it.

## Exporting and printing

**Export** on a page offers:

- **PDF (print)** and **Word (.docx)**, each also **with subpages (handbook)**;
- **Markdown with pictures (.zip)**, also with subpages;
- each table of the text as **CSV**.

If the page's type has a Word template, the Word export uses it. Lists of items and query results export as CSV.

## Process engines

If your administrator has connected a process engine, a text can show its processes as BPMN diagrams, with an operations view of counts, its forms, decisions and architecture sketches, and start forms that start a process when they are sent.

Tasks of an engine can appear as linked tasks in a room. Under **Account and password → Connected engines** you connect your account to an engine; then you take a task on, give it back and complete it with its form directly in Orbithek (**Take on**, **Give back**, **Complete…**). A linked task's title and fields belong to the engine; it ends when the engine ends it.

## AI assistants

**AI access** explains how an AI assistant reads, searches and writes pages over the Model Context Protocol (MCP), always with exactly your rights:

- For assistants on your own computer, create a token (**read and write** or **read only**) and copy the ready-made settings for your client.
- Hosted assistants such as Claude are registered once by an administrator; each person then approves the assistant the first time and may withdraw the approval later.

Moving, deleting, restrictions and administration are deliberately not available to assistants.

## Your account

**Account and password** in the account menu holds:

- your **Profile picture**, shown next to your name;
- the **Language** of the interface;
- the **Look**: as the system, light, dark, or *Cockpit*;
- **Change password**;
- **API tokens** for scripts and tools, each with an expiry; a token is shown only once and can be revoked;
- **Message channels**: a mail address and webhooks for notifications;
- **Connected engines**.

## Governance

The module **Governance** shows, for each room you hold a role in, the findings that need attention:

- **Sensitive data**: items that seem to contain passwords, keys or account numbers;
- **Not changed** for a year (or a chosen number of days);
- **Implausible**: formal contradictions, such as an end before a start;
- **Personal data** in need of protection, where data protection regulations are active;
- **Review**: items whose type asks for a periodic review; **Mark as reviewed** confirms one.

The person who changed an item last is named responsible and sees their open findings in the notifications.

## Administration

Administrators find under **Administration**:

- **Account requests**, **Users and system roles**, **Groups** and **Grant a room role**;
- **Rooms**: create rooms, open their settings, move a room to the trash and restore or delete it for good;
- **Item types**: create and change types in a form — name, text, fields with their kind and options, workflow with states and transitions (or simply *open → done*) and start text — for the whole installation or for one room; the type as JSON stays under **Advanced**. Types are also made from the ready-made templates;
- **Backup** (also on a schedule to a directory, WebDAV, S3, FTPS or SFTP) and **Storage**;
- **Calendars of other services**, **Album: older versions**, **Connectors** and **Process engines**;
- **Appearance (corporate design)**: logo, colours, background, fonts, and a ready-made theme, among them *Capacity*, a quiet look for notes in light and dark;
- **Word templates**, **Data protection regulations**, **Import** and the **Audit trail**.

Room administrators manage members and roles of their room under the room's **Settings**. There, under **Item types of this room**, they also define, change and remove the room's own types with the same form, or start from a ready-made template. A type is removed only when no item and no kanban uses it.

## Keyboard shortcuts

| Keys | Action |
| --- | --- |
| Ctrl+B, Ctrl+I, Ctrl+U | Bold, italic, underline |
| Ctrl+Shift+X, Ctrl+Shift+H | Strike through, highlight |
| Ctrl+Shift+L, E, R, J | Align left, centre, right, justify |
| Ctrl+K | Insert or edit a link; outside a text, open the command palette |
| Ctrl+Shift+P | Open the command palette |
| Alt+Shift+D | Open today's daily note |
| Shift+click | Open a linked item beside the current one; Esc closes it |
| @ | Mention a person or an item |
| Ctrl+Z, Ctrl+Y | Undo, redo |
| Ctrl+Shift+V | Paste as plain text |
| Ctrl+Enter | Run a query |
| Ctrl+Shift+F, F11 | Reading mode; Esc leaves it |
| Shift+arrow | Move a calendar entry |

On a Mac, use Cmd instead of Ctrl.

## Getting help

If a function described here is missing, it may be switched off on your server or your role may not allow it; ask your room administrator or an administrator of the installation. The start page shows the version of Orbithek you are using.
