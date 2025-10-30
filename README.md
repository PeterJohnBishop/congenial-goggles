# congenial-goggles

Upload a file to S3 with a user defined 'shared secret'. 

File metadata is stored in DynamoDB.

Download the file directly only if the shared secret sent in the request Form data 
and original file attribute pulled from DynamoDB generate a matching HMAC.

Optionally, request a presigned download URL to retrieve the file directly from S3 
for a limited time period. 

Optionally, request a presigned download URL rendered as a QR code to retrieve the 
file directly from S3 for a limited time period. 

Endpoints are ratelimited through middleware.

Upload and download transactions are streamed to lower memory usage and increase 
the allowable file size (5GB uploads currently).

Every incoming HTTP request to a Gin server is automatically handled in its own 
goroutine, so upload or download requests run concurrently and don't block other 
incomming requests.

IDEA:

Modify this so that it can be an authentication server that allows users to be created, authenticated, upload an avatar, and get email password resets and other notifications.