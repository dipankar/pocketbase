# File Management

PocketBase provides built-in file storage and management for your collections.

## File Fields

Add file fields to collections to enable uploads:

```json
{
  "name": "attachments",
  "type": "file",
  "options": {
    "maxSelect": 5,
    "maxSize": 10485760,
    "mimeTypes": ["image/jpeg", "image/png", "application/pdf"],
    "thumbs": ["100x100", "400x0"],
    "protected": false
  }
}
```

### Field Options

| Option | Type | Description |
|--------|------|-------------|
| `maxSelect` | int | Maximum number of files (1 = single file) |
| `maxSize` | int | Maximum file size in bytes |
| `mimeTypes` | array | Allowed MIME types (empty = all) |
| `thumbs` | array | Thumbnail sizes for images |
| `protected` | bool | Require token for access |

## Uploading Files

### Single File

```bash
POST /api/collections/posts/records
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="title"

My Post
--boundary
Content-Disposition: form-data; name="cover"; filename="image.jpg"
Content-Type: image/jpeg

<binary data>
--boundary--
```

### Multiple Files

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

### JavaScript SDK

```javascript
// With File object
const formData = new FormData();
formData.append('title', 'My Post');
formData.append('cover', fileInput.files[0]);

const record = await pb.collection('posts').create(formData);

// With multiple files
for (const file of fileInput.files) {
    formData.append('images', file);
}

// With Blob
const blob = new Blob(['Hello World'], { type: 'text/plain' });
formData.append('document', blob, 'hello.txt');
```

## Accessing Files

Files are served from:

```
/api/files/{collectionIdOrName}/{recordId}/{filename}
```

### Get File URL

```javascript
// Using SDK
const url = pb.files.getURL(record, record.cover);
// => http://127.0.0.1:8090/api/files/posts/abc123/image.jpg

// Manual construction
const url = `${baseUrl}/api/files/${record.collectionId}/${record.id}/${record.cover}`;
```

### Thumbnails

For image files, request specific sizes:

```
# Width x Height (crop to fit)
/api/files/posts/abc123/photo.jpg?thumb=100x100

# Width only (proportional)
/api/files/posts/abc123/photo.jpg?thumb=200x0

# Height only (proportional)
/api/files/posts/abc123/photo.jpg?thumb=0x200

# Fit within dimensions (no crop)
/api/files/posts/abc123/photo.jpg?thumb=200x200f

# Crop from top
/api/files/posts/abc123/photo.jpg?thumb=200x200t

# Crop from bottom
/api/files/posts/abc123/photo.jpg?thumb=200x200b
```

```javascript
// SDK
const thumbUrl = pb.files.getURL(record, record.cover, { thumb: '100x100' });
```

### Protected Files

For fields with `protected: true`:

```javascript
// Get file token
const token = await pb.files.getToken();

// Use token in URL
const url = pb.files.getURL(record, record.document, { token });
// => http://127.0.0.1:8090/api/files/.../document.pdf?token=xxx
```

### Force Download

```
/api/files/posts/abc123/document.pdf?download=1
```

## Updating Files

### Replace File

```javascript
const formData = new FormData();
formData.append('cover', newFile);

await pb.collection('posts').update(record.id, formData);
```

### Append Files

Use `+` suffix for multi-file fields:

```javascript
const formData = new FormData();
formData.append('images+', additionalFile);

await pb.collection('posts').update(record.id, formData);
```

Or with SDK:

```javascript
await pb.collection('posts').update(record.id, {
    'images+': [newFile1, newFile2]
});
```

### Remove Files

Use `-` suffix with filenames:

```javascript
await pb.collection('posts').update(record.id, {
    'images-': ['photo1.jpg', 'photo2.jpg']
});
```

Or clear all:

```javascript
await pb.collection('posts').update(record.id, {
    'cover': ''
});
```

## Storage

### Local Storage

Default storage location: `pb_data/storage/`

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

Configure S3-compatible storage for production:

**Admin UI**: Settings > Files storage

**Or via API:**

```bash
PATCH /api/settings
Authorization: Bearer <superuser_token>
Content-Type: application/json

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

#### Compatible Providers

- AWS S3
- DigitalOcean Spaces
- Backblaze B2
- MinIO
- Cloudflare R2
- Wasabi
- Any S3-compatible service

## MIME Type Restrictions

### Common MIME Types

**Images:**
- `image/jpeg`
- `image/png`
- `image/gif`
- `image/webp`
- `image/svg+xml`

**Documents:**
- `application/pdf`
- `application/msword`
- `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
- `text/plain`

**Audio/Video:**
- `audio/mpeg`
- `audio/wav`
- `video/mp4`
- `video/webm`

**Archives:**
- `application/zip`
- `application/x-rar-compressed`
- `application/gzip`

### Wildcards

```json
{
  "mimeTypes": ["image/*", "application/pdf"]
}
```

## Size Limits

Set maximum file sizes:

```json
{
  "maxSize": 5242880
}
```

Common sizes:
- 1 MB = 1048576 bytes
- 5 MB = 5242880 bytes
- 10 MB = 10485760 bytes
- 50 MB = 52428800 bytes
- 100 MB = 104857600 bytes

## Image Processing

### Pre-defined Thumbnails

Configure in field options for automatic generation:

```json
{
  "thumbs": ["100x100", "300x300", "1000x0"]
}
```

### On-Demand Thumbnails

Any size can be requested if within reasonable limits:

```
?thumb=150x150
?thumb=800x600f
```

### Thumbnail Modes

| Mode | Suffix | Description |
|------|--------|-------------|
| Center crop | `WxH` | Crop from center (default) |
| Fit | `WxHf` | Fit within dimensions |
| Top crop | `WxHt` | Crop from top |
| Bottom crop | `WxHb` | Crop from bottom |

## Security

### File Validation

PocketBase validates:
- File size against `maxSize`
- MIME type against `mimeTypes`
- Number of files against `maxSelect`

### Protected Files

Enable for sensitive documents:

```json
{
  "protected": true
}
```

Protected files require a file token:

```javascript
const token = await pb.files.getToken();
const url = pb.files.getURL(record, filename, { token });
```

Tokens expire after a short period.

### Content-Type Header

Files are served with correct Content-Type headers based on MIME type.

### File Names

Original filenames are preserved but sanitized:
- Special characters removed
- Unique suffix added to prevent collisions

## Best Practices

1. **Set appropriate limits** - Restrict file types and sizes
2. **Use S3 for production** - Better scalability and reliability
3. **Generate thumbnails** - Pre-configure common sizes
4. **Protect sensitive files** - Enable protected mode
5. **Clean up orphaned files** - Files are auto-deleted with records
6. **Monitor storage** - Track usage, especially with local storage
7. **Use CDN** - Consider CDN for high-traffic file serving

## Troubleshooting

### File Upload Fails

- Check file size against `maxSize`
- Verify MIME type is allowed
- Ensure `maxSelect` not exceeded
- Check disk space

### Thumbnail Not Generating

- Verify image is valid format (JPEG, PNG, GIF, WebP)
- Check thumb format is correct (`WxH`)
- Ensure sufficient memory for large images

### S3 Connection Issues

- Verify credentials are correct
- Check endpoint URL format
- Ensure bucket exists and is accessible
- Test with S3 test endpoint in settings

## Next Steps

- [Files API](../api/files.md)
- [Collections](../api/collections.md)
- [Deployment Guide](../guides/deployment.md)
