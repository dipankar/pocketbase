# Troubleshooting

Common issues and their solutions when working with PocketBase.

## Server Issues

### PocketBase Won't Start

**Symptoms:** Binary fails to execute or exits immediately.

**Solutions:**

1. **Check permissions:**
   ```bash
   chmod +x pocketbase
   ```

2. **Check port availability:**
   ```bash
   lsof -i :8090
   # Kill conflicting process or use different port
   ./pocketbase serve --http=0.0.0.0:8091
   ```

3. **Check data directory permissions:**
   ```bash
   ls -la pb_data/
   chown -R $(whoami) pb_data/
   ```

4. **View error output:**
   ```bash
   ./pocketbase serve 2>&1
   ```

### Database Locked

**Symptoms:** "database is locked" errors.

**Solutions:**

1. **Only one instance:** Ensure only one PocketBase instance accesses the database
2. **Check for stale processes:**
   ```bash
   ps aux | grep pocketbase
   kill <pid>
   ```
3. **Remove lock file (if corrupt):**
   ```bash
   # Stop PocketBase first
   rm pb_data/data.db-wal
   rm pb_data/data.db-shm
   ```

### High Memory Usage

**Solutions:**

1. **Reduce query complexity**
2. **Add database indexes**
3. **Limit concurrent connections**
4. **Reduce file upload sizes**

## Authentication Issues

### Can't Login

**Symptoms:** Login fails with valid credentials.

**Solutions:**

1. **Check email is verified** (if verification required)
2. **Reset password:**
   ```bash
   ./pocketbase superuser update admin@example.com newpassword
   ```
3. **Check account isn't banned** (if using custom ban logic)
4. **Verify collection is auth type**

### Token Expired/Invalid

**Solutions:**

1. **Refresh token before expiration**
2. **Check token is for correct collection**
3. **Verify server time is accurate:**
   ```bash
   timedatectl status
   ```
4. **Re-authenticate if token expired**

### OAuth2 Not Working

**Solutions:**

1. **Verify redirect URI matches exactly**
2. **Check client ID and secret are correct**
3. **Ensure provider is enabled**
4. **Check scopes are appropriate**
5. **Verify callback URL is accessible**

## API Issues

### 403 Forbidden

**Causes:**
- API rules blocking access
- Missing or invalid authentication
- Rule condition not met

**Solutions:**

1. **Check API rules:**
   ```
   # Common rule issues
   @request.auth.id != ""  # Requires auth
   @request.auth.verified = true  # Requires verified
   ```

2. **Verify token is included:**
   ```bash
   curl -H "Authorization: Bearer YOUR_TOKEN" ...
   ```

3. **Test with simpler rule temporarily**

### 404 Not Found

**Solutions:**

1. **Check collection exists**
2. **Verify record ID is correct**
3. **Check URL path (case-sensitive)**
4. **Ensure collection name matches API call**

### 400 Bad Request

**Solutions:**

1. **Check request body is valid JSON**
2. **Verify required fields are included**
3. **Check field validation constraints**
4. **Review error message for specific field**

### Slow API Responses

**Solutions:**

1. **Add indexes on filtered fields:**
   ```sql
   CREATE INDEX idx_posts_status ON posts (status)
   ```
2. **Reduce expand depth**
3. **Use pagination**
4. **Enable `skipTotal` for list requests**
5. **Optimize filter expressions**

## File Issues

### File Upload Fails

**Solutions:**

1. **Check file size limit:**
   - Collection field maxSize
   - Reverse proxy limit (client_max_body_size)

2. **Verify MIME type is allowed**

3. **Check disk space:**
   ```bash
   df -h pb_data/
   ```

4. **Verify file field accepts multiple (if uploading multiple)**

### Files Not Accessible

**Solutions:**

1. **Check if field is protected (requires token)**
2. **Verify file URL format:**
   ```
   /api/files/{collection}/{record}/{filename}
   ```
3. **Check file exists in storage**
4. **Verify API rules allow access**

### S3 Connection Issues

**Solutions:**

1. **Verify credentials are correct**
2. **Check endpoint format:**
   ```
   https://s3.us-east-1.amazonaws.com
   ```
3. **Verify bucket exists and is accessible**
4. **Check IAM permissions**
5. **Test with S3 test endpoint in Admin UI**

## Email Issues

### Emails Not Sending

**Solutions:**

1. **Verify SMTP settings:**
   - Host, port, username, password
   - TLS enabled if required

2. **Test email configuration:**
   ```bash
   curl -X POST http://localhost:8090/api/settings/test/email \
     -H "Authorization: Bearer TOKEN" \
     -d '{"email":"test@example.com","template":"verification"}'
   ```

3. **Check spam folder**

4. **Review server logs for SMTP errors**

### Emails Going to Spam

**Solutions:**

1. **Set up SPF record:**
   ```
   v=spf1 include:_spf.your-email-provider.com ~all
   ```

2. **Configure DKIM**

3. **Set up DMARC**

4. **Use transactional email service (SendGrid, Mailgun)**

## Realtime Issues

### WebSocket Not Connecting

**Solutions:**

1. **Check reverse proxy WebSocket support:**
   ```nginx
   proxy_http_version 1.1;
   proxy_set_header Upgrade $http_upgrade;
   proxy_set_header Connection "upgrade";
   ```

2. **Verify CORS settings allow WebSocket origin**

3. **Check firewall allows WebSocket connections**

4. **Test direct connection (bypass proxy)**

### Not Receiving Updates

**Solutions:**

1. **Verify subscription syntax**
2. **Check API rules allow list access**
3. **Ensure record matches subscription filter**
4. **Confirm client is connected:**
   ```javascript
   pb.realtime.subscribe('collection', console.log);
   ```

## Database Issues

### Database Corruption

**Symptoms:** "malformed database" or similar errors.

**Solutions:**

1. **Restore from backup**
2. **Try integrity check:**
   ```bash
   sqlite3 pb_data/data.db "PRAGMA integrity_check;"
   ```
3. **Attempt recovery:**
   ```bash
   sqlite3 pb_data/data.db ".recover" | sqlite3 recovered.db
   ```

### Migration Fails

**Solutions:**

1. **Check error message for specific issue**
2. **Verify collection/field exists**
3. **Check for conflicting data**
4. **Run migrations manually if needed**

## Performance Issues

### High CPU Usage

**Solutions:**

1. **Check for complex queries**
2. **Add appropriate indexes**
3. **Reduce concurrent connections**
4. **Profile with logging**

### Slow Queries

**Solutions:**

1. **Add indexes for filtered fields**
2. **Reduce expand depth**
3. **Use specific field selection**
4. **Paginate large result sets**
5. **Use `skipTotal` parameter**

### Memory Leaks

**Solutions:**

1. **Update to latest PocketBase version**
2. **Check for unclosed resources in hooks**
3. **Monitor with process tools**
4. **Restart periodically (temporary fix)**

## Deployment Issues

### HTTPS Not Working

**Solutions:**

1. **For auto TLS, ensure:**
   - Domain points to server
   - Ports 80 and 443 accessible
   - No other process on port 443

2. **Check certificate validity:**
   ```bash
   openssl s_client -connect example.com:443
   ```

3. **Verify reverse proxy SSL configuration**

### Cannot Access Admin UI

**Solutions:**

1. **Check URL is `/_/` (note trailing slash)**
2. **Verify server is running**
3. **Check firewall allows connection**
4. **Try direct IP access to bypass DNS issues**

## Getting Help

### Collect Information

Before asking for help, gather:

1. PocketBase version: `./pocketbase version`
2. OS and architecture
3. Error messages (full text)
4. Relevant configuration
5. Steps to reproduce

### Resources

- [GitHub Issues](https://github.com/pocketbase/pocketbase/issues)
- [GitHub Discussions](https://github.com/pocketbase/pocketbase/discussions)

### Debug Mode

Enable verbose logging temporarily:

```bash
./pocketbase serve --dev
```

!!! warning "Not for Production"
    `--dev` mode logs sensitive information. Don't use in production.

## Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| `database is locked` | Multiple processes | Stop other instances |
| `record not found` | Invalid ID | Check record exists |
| `validation failed` | Invalid data | Check field requirements |
| `unauthorized` | Missing/invalid token | Re-authenticate |
| `forbidden` | Rule blocks access | Review API rules |
| `rate limit exceeded` | Too many requests | Implement backoff |
| `file too large` | Exceeds limit | Reduce size or increase limit |

## Next Steps

- [Deployment Guide](deployment.md)
- [Production Setup](production.md)
- [API Reference](../api/overview.md)
