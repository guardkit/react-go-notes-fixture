// Generated from contract/openapi.yaml by openapi-typescript 7.13.0.
// Do not edit: run `npm run contract:generate` after changing the contract.

export interface paths {
    "/api/health": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/notes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listNotes"];
        put?: never;
        post: operations["createNote"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/notes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        /** @description The one write a caller may aim at a single note: its done flag. The id comes from the path, the owner from the token, and the flag from the body - nothing else, so the label and the creation time stay the row's own and come back as storage holds them. Setting the flag to what it already is answers the same note. */
        patch: operations["updateNoteDone"];
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        Error: {
            message: string;
        };
        Health: {
            run: string;
            /** @enum {string} */
            status: "ok";
        };
        NewNote: {
            label: string;
        };
        Note: {
            /** Format: date-time */
            createdAt: string;
            done: boolean;
            id: string;
            label: string;
        };
        NoteList: {
            notes: components["schemas"]["Note"][];
        };
        /** @description The whole of what the caller may change on one note: the completion flag, and nothing beside it. The id comes from the path, the owner from the token, and the label and creation time are never parameters of this write - the create path (NewNote) stays the only one that carries a label, and it still carries the label alone. */
        PatchNoteRequest: {
            done: boolean;
        };
    };
    responses: {
        /** @description No note of the signed-in owner carries that id. A note that exists but belongs to someone else is answered identically - the same status, the same body shape, carrying neither the id nor the owner - so the answer cannot be read as a probe of whether another user's note exists. */
        NotFound: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description No token, or a token that is expired, unsigned, for another audience or from an unaccepted issuer. */
        SignInRequired: {
            headers: {
                "WWW-Authenticate": string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description Persistence failed; no detail is disclosed. */
        Unavailable: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
    };
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Process liveness and the run this instance belongs to. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Health"];
                };
            };
        };
    };
    listNotes: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The signed-in owner's notes, newest first. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["NoteList"];
                };
            };
            401: components["responses"]["SignInRequired"];
            500: components["responses"]["Unavailable"];
        };
    };
    createNote: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["NewNote"];
            };
        };
        responses: {
            /** @description The saved note. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Note"];
                };
            };
            /** @description The body or label is not acceptable. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
            401: components["responses"]["SignInRequired"];
            500: components["responses"]["Unavailable"];
        };
    };
    updateNoteDone: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description The id of the note to update, as Note.id carries it. The owner and the id together decide whether a row is touched, so an id that is not one of the caller's notes - never written, belonging to someone else, or not a note id at all - answers the one 404 below. */
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["PatchNoteRequest"];
            };
        };
        responses: {
            /** @description The whole note as storage now holds it, carrying the flag the caller set. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Note"];
                };
            };
            /** @description The body carries no done flag, or one that is not a boolean, or anything beside the flag. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
            401: components["responses"]["SignInRequired"];
            404: components["responses"]["NotFound"];
            500: components["responses"]["Unavailable"];
        };
    };
}
