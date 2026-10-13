-- Notes belong to the token subject (`sub`) that created them.
CREATE TABLE notes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_sub TEXT NOT NULL CHECK (char_length(owner_sub) BETWEEN 1 AND 255),
    label TEXT NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80)
);
CREATE INDEX notes_owner_sub_idx ON notes (owner_sub);
