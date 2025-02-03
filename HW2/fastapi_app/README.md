# FastAPI SQLModel Application

This project is a FastAPI application that implements an API for user authentication and token management using SQLModel for database interactions.

## Project Structure

```
fastapi-sqlmodel-app
├── src
│   ├── main.py          # Entry point of the FastAPI application
│   ├── config.py        # Configuration settings for the application
│   ├── database.py      # Database connection and session management
│   ├── models
│   │   └── user.py      # User model definition
│   └── api
│       └── endpoints.py  # API endpoints for token generation and validation
├── requirements.txt      # Project dependencies
└── README.md             # Project documentation
```

## Setup Instructions

1. **Clone the repository:**
   ```
   git clone <repository-url>
   cd fastapi-sqlmodel-app
   ```

2. **Create a virtual environment:**
   ```
   python -m venv venv
   source venv/bin/activate  # On Windows use `venv\Scripts\activate`
   ```

3. **Install dependencies:**
   ```
   pip install -r requirements.txt
   ```

4. **Set up environment variables:**
   Create a `.env` file in the root directory and add your database connection settings and any other necessary configurations.

5. **Run the application:**
   ```
   uvicorn src.main:app --reload
   ```

## Usage

- **Token Generation:**
  Send a POST request to `/token/` with the required fields: `client_id`, `scope`, `client_secret`, and `grant_type`.

- **Token Validation:**
  Send a GET request to `/check/` with the `Authorization` header set to `Bearer <token>`.

## License

This project is licensed under the MIT License. See the LICENSE file for more details.