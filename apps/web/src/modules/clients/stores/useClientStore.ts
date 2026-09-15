import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type {
  Client,
  ClientContact,
  ClientDeleteImpact,
  ClientRole,
  ClientSignature,
} from "@/modules/clients/types";
import type {
  ClientContactFormValues,
  ClientCreateFormValues,
  ClientMasterFormValues,
  RepresentativeFormValues,
} from "@/modules/clients/schemas/client.schema";
import { toPaginationMeta, EMPTY_PAGINATION_META, type PaginationMeta, type RawPaginationMeta } from "@/shared/types/pagination";

interface RawClient {
  id: number;
  brideName: string;
  groomName: string;
  displayName: string;
  phone: string;
  email: string;
  notes: string;
  contactCount: number;
  projectCount: number;
}

interface RawContact {
  id: number;
  clientId: number;
  role: ClientRole;
  username: string;
  relationNote: string;
  name: string;
  phone: string;
  email: string;
  isActive: boolean;
  lastCredentialResetAt: string | null;
}

function toClient(raw: RawClient): Client {
  return {
    id: String(raw.id),
    brideName: raw.brideName,
    groomName: raw.groomName,
    displayName: raw.displayName,
    phone: raw.phone,
    email: raw.email,
    notes: raw.notes,
    contactCount: raw.contactCount ?? 0,
    projectCount: raw.projectCount ?? 0,
  };
}

function toContact(raw: RawContact): ClientContact {
  return {
    id: String(raw.id),
    clientId: String(raw.clientId),
    role: raw.role,
    username: raw.username,
    relationNote: raw.relationNote,
    name: raw.name,
    phone: raw.phone,
    email: raw.email,
    isActive: raw.isActive,
    lastCredentialResetAt: raw.lastCredentialResetAt,
  };
}

interface ClientState {
  clients: Client[];
  clientsMeta: PaginationMeta;
  currentClient: Client | null;
  contacts: ClientContact[];
  clientSignature: ClientSignature | null;

  fetchClients: (page?: number, search?: string, limit?: number) => Promise<void>;
  fetchClient: (clientId: string) => Promise<void>;
  createClient: (values: ClientMasterFormValues) => Promise<Client>;
  updateClient: (clientId: string, values: ClientMasterFormValues) => Promise<void>;
  fetchDeleteImpact: (clientId: string) => Promise<ClientDeleteImpact>;
  deleteClient: (clientId: string) => Promise<void>;

  fetchContacts: (clientId: string) => Promise<void>;
  createContact: (clientId: string, role: ClientRole, values: ClientCreateFormValues) => Promise<void>;
  updateContact: (clientId: string, contactId: string, values: ClientContactFormValues) => Promise<void>;
  toggleContactActive: (clientId: string, contactId: string) => Promise<void>;
  deleteContact: (clientId: string, contactId: string) => Promise<void>;
  resetContactCredential: (clientId: string, contactId: string, password: string) => Promise<void>;
  replaceRepresentative: (clientId: string, contactId: string, values: RepresentativeFormValues) => Promise<void>;

  fetchClientSignature: (clientId: string) => Promise<void>;
  deleteClientSignature: (clientId: string) => Promise<void>;
}

// Backed by the real `clients` module — master pasangan berpaginasi +
// kontak per client (`/clients/{id}/contacts`). Cross-module reads happen
// via otherStore.getState(), never by importing internals (ADR-0009).
export const useClientStore = create<ClientState>((set, get) => ({
  clients: [],
  clientsMeta: EMPTY_PAGINATION_META,
  currentClient: null,
  contacts: [],
  clientSignature: null,

  fetchClients: async (page = 1, search = "", limit) => {
    const res = await httpClient.get(API.clients.base, { params: { page, search: search || undefined, limit } });
    set({
      clients: (res.data.data as RawClient[]).map(toClient),
      clientsMeta: toPaginationMeta(res.data.meta as RawPaginationMeta),
    });
  },

  fetchClient: async (clientId) => {
    const res = await httpClient.get(API.clients.item(clientId));
    const data = res.data.data as { client: RawClient; contacts: RawContact[] };
    set({ currentClient: toClient(data.client), contacts: data.contacts.map(toContact) });
  },

  createClient: async (values) => {
    const res = await httpClient.post(API.clients.base, values);
    const client = toClient(res.data.data as RawClient);
    await get().fetchClients();
    return client;
  },

  updateClient: async (clientId, values) => {
    await httpClient.patch(API.clients.item(clientId), values);
    await get().fetchClient(clientId);
  },

  fetchDeleteImpact: async (clientId) => {
    const res = await httpClient.get(API.clients.deleteImpact(clientId));
    const raw = res.data.data as {
      client: RawClient;
      contactCount: number;
      projectCount: number;
      projectNames: string[] | null;
      paidInvoiceCount: number;
      paidInvoiceTotal: number;
      quotationCount: number;
      quotationNumbers: string[] | null;
    };
    return {
      client: toClient(raw.client),
      contactCount: raw.contactCount,
      projectCount: raw.projectCount,
      projectNames: raw.projectNames ?? [],
      paidInvoiceCount: raw.paidInvoiceCount,
      paidInvoiceTotal: raw.paidInvoiceTotal,
      quotationCount: raw.quotationCount,
      quotationNumbers: raw.quotationNumbers ?? [],
    };
  },

  deleteClient: async (clientId) => {
    await httpClient.delete(API.clients.item(clientId));
    set((state) => ({
      clients: state.clients.filter((c) => c.id !== clientId),
      currentClient: state.currentClient?.id === clientId ? null : state.currentClient,
    }));
  },

  fetchContacts: async (clientId) => {
    const res = await httpClient.get(API.clients.contacts(clientId));
    set({ contacts: (res.data.data as RawContact[]).map(toContact) });
  },

  createContact: async (clientId, role, values) => {
    await httpClient.post(API.clients.contacts(clientId), {
      role,
      relationNote: values.relationNote,
      name: values.name,
      phone: values.phone,
      email: values.email,
      username: values.username,
      password: values.password,
    });
    await get().fetchContacts(clientId);
  },

  updateContact: async (clientId, contactId, values) => {
    await httpClient.patch(API.clients.contact(clientId, contactId), values);
    await get().fetchContacts(clientId);
  },

  toggleContactActive: async (clientId, contactId) => {
    await httpClient.post(API.clients.contactToggleActive(clientId, contactId));
    await get().fetchContacts(clientId);
  },

  deleteContact: async (clientId, contactId) => {
    await httpClient.delete(API.clients.contact(clientId, contactId));
    await get().fetchContacts(clientId);
  },

  resetContactCredential: async (clientId, contactId, password) => {
    await httpClient.post(API.clients.contactResetCredential(clientId, contactId), { password });
    await get().fetchContacts(clientId);
  },

  replaceRepresentative: async (clientId, contactId, values) => {
    await httpClient.post(API.clients.contactReplaceRepresentative(clientId, contactId), values);
    await get().fetchContacts(clientId);
  },

  fetchClientSignature: async (clientId) => {
    const res = await httpClient.get(API.clients.signature(clientId));
    const raw = res.data.data as {
      role: ClientRole;
      signerName: string;
      source: "draw" | "upload";
      updatedAt: string;
    } | null;
    set({ clientSignature: raw });
  },

  deleteClientSignature: async (clientId) => {
    await httpClient.delete(API.clients.signature(clientId));
    set({ clientSignature: null });
  },
}));
