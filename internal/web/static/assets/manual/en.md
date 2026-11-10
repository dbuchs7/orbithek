# Orbithek user manual

This manual explains how to work with Orbithek: reading and writing pages, keeping track of tasks, working together, and finding what you need. It describes the functions as the user interface shows them. Some functions are available only when your administrator has switched them on; this is said where it applies.

> [!NOTE]
> You open this manual at any time from the menu of your profile picture at the top right: **Manual**. The language links above the text switch between English and German.

## Basic concepts

### Rooms, items and types

Everything people write in Orbithek is an **item**: a wiki page, a task, a decision, a meeting note. What an item consists of is set by its **type**. A type defines the item's fields (for example *assignee*, *due* or *priority*), whether it has a text, and whether it moves through a **workflow** of states such as *open* and *done*. Pages and tasks are therefore not separate systems, but items of different types.

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

- **Nine dots** (Modules): opens the module switcher with Rooms, Create, Tasks, Comments, Album, Whiteboards, Starmap, AI access, Governance, Account and, for administrators, Administration.
- **Start page**, **Back** and **Forward**.
- The **logo**, which also leads to the start page.
- The **search** field.
- **+ New**: creates a page or item where you are.
- The **bell**: your notifications, with the number of unread ones.
- **DE** and **EN**: the language of the interface.
- The **light and dark mode** switch.
- **Reading mode** (Ctrl+Shift+F, F11): the page fills the screen; Esc leaves it.
- Your **profile picture** or initials: opens the account menu with *Account and password*, *AI access*, *Manual*, *Administration* (administrators only) and *Log out*.

### The start page

The start page greets you, shows counts of pages, tasks and pictures, and offers quick access to your tasks, notifications, personal notes, the starmap, the album, the whiteboards and AI access. Below it lists your rooms, as tiles or as a list; a star marks a room as a favourite. If there are no rooms yet, administrators find the first steps there: create a room, add types from the ready-made templates (page, task, decision), and give people a role.

## Pages and items

### Creating a page or item

Choose **+ New** in the top bar or **New page** in a room. The creation view asks for the room, the type and the title, and places the new item below the page you came from. On a page, **New child page** creates a page below it. If a type has a start text, a new item begins with it.

### Reading an item

An item shows its title, its key, type and state, and its text. The panel **About this item** at the side holds its details:

- **Owner and details**: the item's fields, editable by those who may edit.
- **Status**: the current state and the available transitions (**Change the status**).
- **Labels**, **Relations** and **Files**.
- **Access**: who may read and edit.
- **History**: every change of the item.

Above the text, **Export** and **Watch** are available; the comments follow below the text.

### Editing a text

Choose **Edit** to edit the text and **Done** to finish. Changes are saved continuously; the status line shows *Saving…* and then *All changes saved.* The toolbar offers:

- the block kind: paragraph, heading 1 to 3, bulleted, numbered and task items, quote, code block, and info, note, warning, tip and error panels;
- bold, italic, underline, strike-through, highlight, inline code and alignment;
- **Link** (Ctrl+K): a web address, or a page or task found by its title;
- **Mention** (@): a person or an item;
- **Undo** and **Redo**.

Markdown that you paste becomes formatted text. Ctrl+Shift+V pastes plain text instead.

### Inserting content

The menu **Insert…** adds blocks after the cursor:

- **Table**, **Divider**, **Table of contents**, **List of figures**, **List of tables** and **Child pages**;
- **Image**, **Video**, **Drawing**, a picture **From the album** or a **Copied picture**;
- **Embed an item…**: shows another item whole inside the text;
- **New task…** (created below the page and shown as a link or whole) and **Task list…** (every task with a label, or the result of a query);
- **Paste Markdown…** and **Markdown file from a repository…** (if enabled);
- BPMN diagrams, and processes, forms, decisions, start forms and architecture sketches of a process engine (see *Process engines*).

Pictures can be placed left or right with text beside them, centred, and resized by dragging; a caption numbers them in the list of figures.

### Labels, relations and files

**Labels** are short words that group items across rooms, such as `release-2` or `security`. Add them in the panel at the side; a click on a label lists every item carrying it and offers it **As a board**.

**Relations** link an item to another with a named type, for example *blocks* or *depends on*: enter the relation and the other item's key and choose **Relate**. Links and mentions in the text are shown as relations too.

**Files** are attached with **Upload files**. Whoever may edit the item renames a file in the list with **Rename**: every version takes the new name, and the file type stays. Pictures and drawings of every item also appear in the album.

### Moving, copying and deleting

- **Move…** puts a page with its child pages below another page or at the top of the room. Within a room, **Move up** and **Move down** change the order.
- **Copy…** copies a page, optionally **With its child pages**.
- **Move to trash** removes an item from sight. It is possible only when no other item outside its subtree refers to it. From the trash it can be restored with **Restore**, or erased with **Delete for good**, which cannot be undone.

### History and versions

**History** lists every change of an item, who made it and when. A version of the text shows its **Changes**; **Restore this version** brings an earlier text back as a new version. Nothing is lost by restoring.

## Working together

### Comments

Every item has comments at its end. Write a comment, mention people with @name, and use **Reply** and **Resolve**; resolved threads stay visible under *Resolved* and can be reopened. To comment on a passage, select text within one block and choose **Comment**; the comment shows the passage, and **Show this text in the page** leads back to it.

The module **Comments** gathers comments on the items you watch, your own and those that mention you (*All*, *Watched*, *Mine*, *Mentioning me*).

### Watching and notifications

**Watch** on a page or a room tells you of what happens there: new child pages, edits, changes of state and fields, comments, labels and files. The bell in the top bar leads to your **Notifications**, filtered by *Mentions*, *Comments* and *Changes*, or *Unread only*. You are also told when someone mentions you, answers an access request or when a page you saved was restricted because of sensitive data. Under **Account and password** you set the channels through which messages reach you, such as a mail address.

### Personal notes

Select a passage while reading and choose **Note** to keep a note on it that only you see. **My notes** on the start page lists all your notes with the passages they belong to.

### Editing at the same time

Several people can edit a text at once. Each block someone is working on is locked for the others; the line above the text names who is in the text right now and who is editing which block. If the connection is lost, the editor reconnects on its own.

## Finding information

### Search

Type words of a title, a text or a comment, or a key such as `PLAT-12`, into the search field. Suggestions appear while you type; **Enter** shows all results, ordered by relevance (one to five stars). The search finds only what you may read.

### Queries

A **query** selects items by their fields. Open it from the search results (**Query the fields of the items instead**). Conditions are joined with AND, OR and NOT; parentheses group them:

- Compare with `=`, `!=`, `<`, `<=`, `>`, `>=`; `~` finds every word of a value; `IN (a, b)` is one of a list.
- `IS EMPTY` and `IS NOT EMPTY` find items without or with a value.
- Dates are written `2026-10-01`; `-7d`, `2w`, `+12h` and `-1y` count from now; `now()`, `startOfWeek()` and similar functions are available.
- `currentUser()` stands for you.
- `ORDER BY` sorts, `ASC` or `DESC`.

Examples:

```
type = task AND assignee = currentUser() AND finished = false ORDER BY due
updated >= -7d ORDER BY updated DESC
text ~ "release" AND label IN (web, security)
```

Ctrl+Enter runs a query. The results can be exported as CSV.

### Views of a room

A room offers several views: its page tree, **All items**, a **Board** per type with a workflow, the **Calendar**, the **Roadmap** and, depending on your role, **Governance** and **Link check**. **Watch** on a room tells you of everything that happens in it.

## Tasks

### The task list

The module **Tasks** lists open work across your rooms: items of the type *task* and of every type with a workflow and an *assignee* or *due* field. The tabs show **Assigned to me**, **Unassigned** and **All open**; filters narrow by room, state, person, due date and labels. **Take over** assigns a task to you, **Give back** releases it. Overdue tasks are marked.

### Boards and kanbans

A **board** shows the items of one type in a room as columns of their workflow states. Dragging a card to another column carries out the workflow's transition.

A **kanban** gathers the tasks of a topic: choose **New kanban** under *Tasks → Kanbans*, give it a name, the type of its cards and optionally a label. Every card on it carries the label. In the **Board settings** you choose:

- the **Columns**: state columns move a card through the workflow; own columns keep its state;
- a **Limit** of cards per column, which marks an overfull column;
- the fields **Shown on the cards**, the **Colour of the cards** by a choice field, and **Swimlanes** by a person or choice field.

Cards are added with **New card**, opened for their details, and taken off with **Take off this kanban**. **Boards by label** shows every label's tasks as a board without creating a kanban.

### Roadmap and Gantt chart

**Roadmap** shows items with date fields on a time axis: as a **Roadmap by label** in lanes, or as a **Gantt chart** with one row per item. Choose the room, the types, the **Start** and **End** fields, the labels and the **Scale** (weeks, months, quarters). Items that wait on unfinished items are marked.

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
- **Map**: the places where pictures were taken.

Search by name, label, key or title, and narrow by kind, room and label. The search also takes a date and finds the pictures taken then, or added then when a picture tells no time: a year (*2026*), a month (*11.2026*, *2026-11*, *November 2026*), a day (*10.11.2026*, *2026-11-10*, *10 November 2026*), or a day or month without a year (*10.11.*, *November*), which finds it in every year.

### Uploading and importing

Pictures are added where you are, in the **Folders** view:

- *In a folder*, choose **Upload pictures**, drop pictures and ZIP archives onto the album, or paste a picture with Ctrl+V, such as a screenshot. An empty folder shows a large area for this. **Add from the album…** copies pictures that are already in the album into the folder; the original stays where it is.
- *In a collection*, the same ways work. A collection holds no files of its own, so the album asks which folder the new pictures are kept in, the folder chosen last coming first, or makes a **New folder…**; each picture gets the collection's labels and so shows in the collection. **Add from the album…** gives pictures already in the album the collection's labels, without copying them.
- *At the top* of the album, uploading, dropping or pasting asks for the folder as well, and that folder then opens.

A PowerPoint presentation comes in with the pictures of its slides (see *Importing a PowerPoint presentation*). Pictures exported from photo apps are unpacked in the browser. If enabled, photos also come **From Google Photos** or **From a shared iCloud album**. HEIC photos are not accepted; export them as JPEG first. Videos from Google Photos and shared iCloud albums are still skipped.

A folder's **Place on import** decides what happens to the place where a photo was taken: **Keep for every reader**, **Keep to me only** or **Remove from the file**. Per picture, the place can later be kept to you or shown to every reader.

Where a picture tells no place, or a wrong one, whoever may edit it sets the **Place** by hand: in the viewer, or for several chosen pictures at once. Typing a name such as *Zermatt* suggests places, unless an administrator has turned the suggestions off; **Choose on the map** sets the place with a click on the map. **Remove the place** clears it.

### Videos

Videos come into the album the same ways as pictures: uploaded, dropped, pasted or in a ZIP archive, as MP4, WebM or MOV (iPhone), each up to 2 GB unless an administrator set another limit. The album then handles them like pictures: in folders and collections, on the timeline and the map, as favourites and memories, with labels, renaming, moving, deleting and the ZIP download.

- When a video is uploaded, the browser takes a picture from it, which the album shows on the card with the video's length. A video the browser cannot play keeps a symbol instead.
- When and where it was taken, how long it plays and the camera are read from the video itself; **Place on import** works as for photos.
- A click plays the video in the viewer. Whether a MOV video plays depends on the browser: Safari plays it, other browsers often only when it was recorded as H.264 rather than HEVC. Otherwise choose **Download**.
- The kind filter **Videos** shows only videos. Videos are not shown as slides when presenting; they are not exported as PNG or JPEG.

### Deleting a folder

**Delete folder** in an open folder moves it to the trash with its folders, pictures and videos, after saying how many go with it. Its page restores it with **Restore**. A folder is not deleted while something outside it, such as a page, still uses what is in it; the album names it.

### Importing a PowerPoint presentation

A presentation becomes a folder of slides, ready to present with its speaker's notes:

1. In PowerPoint, choose **File › Export › Change File Type › PNG** (or JPEG) and export **All Slides**. PowerPoint writes one picture per slide, such as *Slide1.PNG* or *Folie1.PNG*.
2. In the album, open the folder the presentation belongs in and choose **Upload pictures**. Choose the *.pptx* file together with all these pictures, or together with a ZIP archive of them; dropping them onto the folder works too.

A folder named after the presentation is made in the open folder and opens. Each slide becomes a picture named by its place and title, such as *03 Our goals.png*, so the album keeps the order of the slides, and the slide's notes become the picture's speaker's notes. Hidden slides are left out, as a slide show leaves them out.

A presentation file holds no pictures of its slides; this is why the exported pictures are needed. The pictures are matched to the slides by the numbers in their names. If they do not match, nothing is brought in and the album says how many slides and pictures it found. Only the pictures and the notes reach the server, not the presentation file. Animations, transitions, videos and the presentation's own comments are not taken over; *.ppt* files of older versions must first be saved as *.pptx*.

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

The album presents the pictures it shows as slides: open a folder or a collection, narrow it with the search or a label if you like, and choose **Present N pictures**. The slides are ordered by name, numbers by their value, so *2 Plan* comes before *10 Outlook*; to change the order, rename the pictures.

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

## Drawings and whiteboards

### Drawings

**Insert… → Drawing** creates a drawing in the text. The drawing editor offers palettes for ArchiMate, BPMN, network and IT symbols. A drawing is saved as a picture of its page, with all its versions.

### Live whiteboards

Every drawing can be opened with **Open as whiteboard**: up to 20 people and AI agents draw on it at the same time, each element locked while someone works on it. A whiteboard offers coloured notes (**Note**), a **Vote** with a number of votes per person (the result is written as a comment), a **Timer** and **Present**, which others follow frame by frame. Comments can be placed on single elements. The module **Whiteboards** lists the whiteboards in use and the drawings you can open; **New whiteboard** creates one.

While people work on a whiteboard, the drawing cannot be edited alone; the page offers to join instead.

## Maps of connections

### Starmap of a room

The **Starmap** draws a room's items, pictures and shared labels as a map: circles for items (coloured by type), squares for pictures, diamonds for labels, dashed outlines for items of other rooms. Lines show the page tree, relations, mentions, embeds and copies. Drag the map to move it and a node to pull it with its neighbours; turn the mouse wheel to zoom. Choose a node to see its panel. A double-click on a node (or Shift+Enter) shows only it with its neighbourhood at a chosen **Depth** (**Separate**); Escape steps back.

Choose **All rooms** in the room list to see the items of every room you may read on one map, with their tree, relations and shared labels. When they are more than one map shows, each bubble stands for the items of one type in one room, as big as their number, and lines count the links between them; a bubble opens its room's map with **Open the room's map**. **Show a label in every room** narrows the map to the items that carry a label, in every room; its suggestions are the labels in use.

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
- **Item types**: create types from the ready-made templates or as JSON, with fields, workflow, Word template and start text;
- **Backup** (also on a schedule to a directory, WebDAV, S3, FTPS or SFTP) and **Storage**;
- **Calendars of other services**, **Album: older versions**, **Connectors** and **Process engines**;
- **Appearance (corporate design)**: logo, colours, background, fonts;
- **Word templates**, **Data protection regulations**, **Import** and the **Audit trail**.

Room administrators manage members and roles of their room under the room's **Settings**.

## Keyboard shortcuts

| Keys | Action |
| --- | --- |
| Ctrl+B, Ctrl+I, Ctrl+U | Bold, italic, underline |
| Ctrl+Shift+X, Ctrl+Shift+H | Strike through, highlight |
| Ctrl+Shift+L, E, R, J | Align left, centre, right, justify |
| Ctrl+K | Insert or edit a link |
| @ | Mention a person or an item |
| Ctrl+Z, Ctrl+Y | Undo, redo |
| Ctrl+Shift+V | Paste as plain text |
| Ctrl+Enter | Run a query |
| Ctrl+Shift+F, F11 | Reading mode; Esc leaves it |
| Shift+arrow | Move a calendar entry |

On a Mac, use Cmd instead of Ctrl.

## Getting help

If a function described here is missing, it may be switched off on your server or your role may not allow it; ask your room administrator or an administrator of the installation. The start page shows the version of Orbithek you are using.
