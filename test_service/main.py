import os
from http.server import HTTPServer, BaseHTTPRequestHandler


class Handler(BaseHTTPRequestHandler):

	def do_GET(self):
		response_text = bytes(os.environ.get("RESPONSE_TEXT"), encoding='utf-8')
		self.send_response(200)
		self.send_header('Content-Type', 'text/html')
		self.end_headers()
		self.wfile.write(response_text)


def start_server():
	server_address = ("", 8000)
	server = HTTPServer(server_address, Handler)
	
	try:
		server.serve_forever()
	except KeyboardInterrupt:
		return


if __name__ == '__main__':
	start_server()
