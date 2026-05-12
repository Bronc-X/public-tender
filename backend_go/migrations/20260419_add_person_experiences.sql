CREATE TABLE person_education (
    id TEXT PRIMARY KEY,
    person_id TEXT NOT NULL,
    start_date TEXT,
    end_date TEXT,
    school TEXT,
    degree TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(person_id) REFERENCES person(id) ON DELETE CASCADE
);

CREATE INDEX idx_person_education_person_id ON person_education(person_id);

CREATE TABLE person_work_experience (
    id TEXT PRIMARY KEY,
    person_id TEXT NOT NULL,
    start_date TEXT,
    end_date TEXT,
    company TEXT,
    position TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(person_id) REFERENCES person(id) ON DELETE CASCADE
);

CREATE INDEX idx_person_work_experience_person_id ON person_work_experience(person_id);
