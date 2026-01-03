# Files API

PocketBase handles file uploads and storage for file fields in collections.

## Uploading Files

Files are uploaded when creating or updating records using `multipart/form-data`:

```bash
POST /api/collections/{collection}/records
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="title"

My Document
--boundary
Content-Disposition: form-data; name="document"; filename="report.pdf"
Content-Type: application/pdf

<binary data>
--boundary--
```

### Multiple Files

For multi-file fields, upload multiple files with the same field name:

```bash
--boundary
Content-Disposition: form-data; name="images"; filename="photo1.jpg"
Content-Type: image/jpeg

<binary data>
--boundary
Content-Disposition: form-data; name="images"; filename="photo2.jpg"
Content-Type: image/jpeg

<binary data>
--boundary--
```

## Accessing Files

Files are accessed via the files endpoint:

```
GET /api/files/{collectionIdOrName}/{recordId}/{filename}
```

### Example URLs

```
# Original file
http://127.0.0.1:8090/api/files/posts/abc123/document.pdf

# With token for protected files
http://127.0.0.1:8090/api/files/posts/abc123/document.pdf?token=<file_token>
```

## File Tokens

Protected files require a file token for access:

```bash
POST /api/files/token
Authorization: Bearer <auth_token>
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

Use the token as a query parameter:

```
http://127.0.0.1:8090/api/files/posts/abc123/document.pdf?token=<file_token>
```

## Image Thumbnails

For image files, request thumbnails using the `thumb` parameter:

```
# Width x Height
http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg?thumb=100x100

# Width only (proportional height)
http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg?thumb=200x0

# Height only (proportional width)
http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg?thumb=0x200
```

### Thumbnail Modes

Add mode suffix to control resizing behavior:

| Mode | Description |
|------|-------------|
| `t` (top) | Crop from top |
| `b` (bottom) | Crop from bottom |
| `f` (fit) | Fit within dimensions |
| `WxH` | Exact dimensions (default, crop center) |

```
# Fit within 200x200
http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg?thumb=200x200f

# Crop from top
http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg?thumb=200x200t

# Crop from bottom
http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg?thumb=200x200b
```

### Pre-defined Thumbnails

Configure allowed thumbnail sizes in the collection field options to enable on-the-fly generation:

```json
{
  "name": "avatar",
  "type": "file",
  "options": {
    "thumbs": ["100x100", "200x200", "50x50f"]
  }
}
```

## Downloading Files

Force download with the `download` parameter:

```
http://127.0.0.1:8090/api/files/posts/abc123/document.pdf?download=1
```

## Updating Files

### Replace File

Upload a new file with the same field name:

```bash
PATCH /api/collections/{collection}/records/{id}
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="document"; filename="new_report.pdf"
Content-Type: application/pdf

<binary data>
--boundary--
```

### Append Files (Multi-file Fields)

Use `+` suffix to append:

```bash
PATCH /api/collections/{collection}/records/{id}
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="images+"; filename="photo3.jpg"
Content-Type: image/jpeg

<binary data>
--boundary--
```

### Remove Files

Use `-` suffix with filename to remove:

```bash
PATCH /api/collections/{collection}/records/{id}
Content-Type: application/json

{
  "images-": ["photo1.jpg", "photo2.jpg"]
}
```

Or set to empty:

```json
{
  "document": ""
}
```

## File Field Configuration

Configure file fields in collection schema:

```json
{
  "name": "attachments",
  "type": "file",
  "options": {
    "maxSelect": 10,
    "maxSize": 10485760,
    "mimeTypes": [
      "image/jpeg",
      "image/png",
      "image/gif",
      "application/pdf"
    ],
    "thumbs": ["100x100", "400x0"]
  }
}
```

### Options

| Option | Description |
|--------|-------------|
| `maxSelect` | Maximum number of files (1 for single file) |
| `maxSize` | Maximum file size in bytes |
| `mimeTypes` | Allowed MIME types (empty = all) |
| `thumbs` | Pre-defined thumbnail sizes |
| `protected` | Require file token for access |

## Storage

### Local Storage

By default, files are stored in `pb_data/storage/`:

```
pb_data/
└── storage/
    └── <collection_id>/
        └── <record_id>/
            ├── original_file.jpg
            └── thumbs/
                ├── 100x100_original_file.jpg
                └── 200x200_original_file.jpg
```

### S3 Storage

Configure S3-compatible storage in settings:

```json
{
  "s3": {
    "enabled": true,
    "bucket": "my-bucket",
    "region": "us-east-1",
    "endpoint": "https://s3.amazonaws.com",
    "accessKey": "AKIAIOSFODNN7EXAMPLE",
    "secret": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    "forcePathStyle": false
  }
}
```

## JavaScript SDK Usage

```javascript
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

// Upload file
const formData = new FormData();
formData.append('title', 'My Post');
formData.append('image', fileInput.files[0]);

const record = await pb.collection('posts').create(formData);

// Get file URL
const url = pb.files.getURL(record, record.image);
// => http://127.0.0.1:8090/api/files/posts/abc123/photo.jpg

// Get thumbnail URL
const thumbUrl = pb.files.getURL(record, record.image, { thumb: '100x100' });

// Get protected file URL
const token = await pb.files.getToken();
const protectedUrl = pb.files.getURL(record, record.document, { token });

// Update with new file
const updatedRecord = await pb.collection('posts').update(record.id, {
    image: newFileInput.files[0]
});

// Append files
await pb.collection('posts').update(record.id, {
    'images+': additionalFiles
});

// Remove files
await pb.collection('posts').update(record.id, {
    'images-': ['photo1.jpg']
});
```

## Direct URL Construction

Construct file URLs directly:

```javascript
function getFileUrl(record, filename, thumb = '') {
    const base = 'http://127.0.0.1:8090';
    let url = `${base}/api/files/${record.collectionId}/${record.id}/${filename}`;

    if (thumb) {
        url += `?thumb=${thumb}`;
    }

    return url;
}

// Usage
const url = getFileUrl(record, record.avatar, '100x100');
```

## Error Handling

### File Too Large

```json
{
  "code": 400,
  "message": "Failed to create record.",
  "data": {
    "document": {
      "code": "validation_file_size_limit",
      "message": "The file size must be less than 10MB."
    }
  }
}
```

### Invalid MIME Type

```json
{
  "code": 400,
  "message": "Failed to create record.",
  "data": {
    "document": {
      "code": "validation_invalid_mime_type",
      "message": "The file type is not allowed."
    }
  }
}
```

### Max Files Exceeded

```json
{
  "code": 400,
  "message": "Failed to create record.",
  "data": {
    "images": {
      "code": "validation_max_select_constraint",
      "message": "Select no more than 5 items."
    }
  }
}
```

## Best Practices

1. **Set appropriate limits** - Configure maxSize and maxSelect based on use case
2. **Restrict MIME types** - Only allow necessary file types
3. **Use thumbnails** - Generate thumbnails for images to reduce bandwidth
4. **Protect sensitive files** - Enable protected mode for private documents
5. **Use S3 for scale** - Configure S3 storage for production deployments
6. **Clean up orphaned files** - Deleted records automatically clean up files

## Next Steps

- [Records API](records.md)
- [Collections API](collections.md)
- [File Management Feature](../features/file-management.md)
