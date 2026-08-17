package main

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "runtime"
    "strings"
)

func generateSecureSecrets(bytes int) (string, error) {
    key := make([]byte, bytes)
    _, err := rand.Read(key)
    if err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(key), nil
}

func updateEnvFile(fileName, key, value string) error {
    content, err := os.ReadFile(fileName)
    if err != nil && !os.IsNotExist(err) {
        return err
    }

    lines := strings.Split(string(content), "\n")
    found := false
    newLines := []string{}

    for _, line := range lines {
        if strings.HasPrefix(line, key+"=") {
            newLines = append(newLines, fmt.Sprintf("\n%s=%s\n", key, value))
            found = true
        } else if line != "" {
            newLines = append(newLines, line)
        }
    }

    if !found {
        newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
    }

    return os.WriteFile(fileName, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func main() {
    secret, err := generateSecureSecrets(64)
    if err != nil {
        log.Fatal("Failed to generate secrets:", err)
    }

    fmt.Println("Generated JWT Secret:", secret)

    // Resolve path relative to THIS source file, not the working directory
    _, filename, _, ok := runtime.Caller(0)
    if !ok {
        log.Fatal("Could not determine source file path")
    }

    // Goes up from pkg/secrets/ to project root
    envPath := filepath.Join(filepath.Dir(filename), "..", "..", ".env")
    envPath = filepath.Clean(envPath)

    fmt.Println("Writing to:", envPath) // helpful for debugging

    err = updateEnvFile(envPath, "JWT_SECRETS", secret)
    if err != nil {
        log.Fatal("Failed to update environment file:", err)
    }

    fmt.Println("JWT_SECRET successfully added to .env file")
}