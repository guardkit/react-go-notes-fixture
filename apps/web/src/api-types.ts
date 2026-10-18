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
        patch: operations["setNoteDone"];
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        DoneUpdate: {
            done: boolean;
        };
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
    };
    responses: {
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
    setNoteDone: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description The id of the note whose completion flag is set, addressed through the signed-in owner. */
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["DoneUpdate"];
            };
        };
        responses: {
            /** @description The note as storage holds it now, carrying the flag this caller just wrote. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Note"];
                };
            };
            401: components["responses"]["SignInRequired"];
            /** @description No note of the signed-in owner has this id. A note held by another subject is answered identically, so existence is never disclosed. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
            500: components["responses"]["Unavailable"];
        };
    };
}
