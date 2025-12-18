import fs from "node:fs";
import { normText, safeStat, suggestClosest, ukNumFromCampusName } from "./util.js";

const DAY_ALIASES = new Map([
  ["понедельник", 0],
  ["вторник", 1],
  ["среда", 2],
  ["четверг", 3],
  ["пятница", 4],
  ["суббота", 5],
  ["воскресенье", 6]
]);

const TEACHER_RE =
  /(?:^|[,\s;])(?:(?:доц|проф|асс|ст\.\s*пр|преп)\.?\s+)?([А-ЯЁ][а-яё-]+)\s*([А-ЯЁ])\.\s*([А-ЯЁ])\.(?:$|[,\s;])/gu;

const ROOM_RE =
  /(?:^|[,\s;])(?:ауд\.?\s*)?(\d{1,4}[а-яa-z]?)(?:$|[,\s;])/giu;

const UK_IN_SUBJECT_RE = /(УК\s*№?\s*(\d+)|ук\s*№?\s*(\d+))/iu;

function parseTeacher(subject) {
  const s = String(subject || "");
  const m = [...s.matchAll(TEACHER_RE)];
  if (m.length === 0) return null;
  const last = m[m.length - 1];
  return `${last[1]} ${last[2]}.${last[3]}.`; // "Иванов И.И."
}

function parseRoom(subject) {
  const s = String(subject || "");
  const m = [...s.matchAll(ROOM_RE)];
  if (m.length === 0) return null;
  return String(m[m.length - 1][1]).toUpperCase();
}

function parseUkNumFromSubject(subject) {
  const s = String(subject || "");
  const m = s.match(UK_IN_SUBJECT_RE);
  if (!m) return null;
  return String(m[2] || m[3] || "").trim() || null;
}

export class SnapshotRepo {
  constructor(snapshotPath) {
    this.snapshotPath = snapshotPath;
    this.mtimeMs = 0;

    // groupName -> { meta: { campusName, ukNum, institute, form }, days: Map(weekday -> lessons[]) }
    this.groups = new Map();
    this.groupList = [];

    // choices for UK buttons
    this.ukNums = []; // ["1","2","3"]

    // teacherKey(norm display) -> { display, items[] }
    this.teacherIndex = new Map();
    // roomKey: "401" -> { display:"401", items[] } across all UK
    this.roomIndexGlobal = new Map();
    // roomKey: "2/401" -> { display:"2/401", items[] }
    this.roomIndexByCampus = new Map();
  }

  loadIfPresent() {
    const st = safeStat(this.snapshotPath);
    if (!st) return false;

    const raw = fs.readFileSync(this.snapshotPath, "utf8");
    const data = JSON.parse(raw);
    this._build(data);
    this.mtimeMs = st.mtimeMs;
    return true;
  }

  maybeReload() {
    const st = safeStat(this.snapshotPath);
    if (!st) return false;
    if (st.mtimeMs <= this.mtimeMs) return false;
    return this.loadIfPresent();
  }

  snapshotVersion() {
    return String(Math.floor(this.mtimeMs || 0));
  }

  getGroup(groupName) {
    return this.groups.get(groupName) || null;
  }

  findGroups(query, limit = 10) {
    return suggestClosest(query, this.groupList, limit);
  }

  suggestTeachers(query, limit = 10) {
    const displays = Array.from(this.teacherIndex.values()).map((v) => v.display);
    return suggestClosest(query, displays, limit);
  }

  suggestRooms(roomQuery, ukNumOrNull, limit = 10) {
    if (ukNumOrNull) {
      const displays = Array.from(this.roomIndexByCampus.values())
        .map((v) => v.display)
        .filter((d) => d.startsWith(`${ukNumOrNull}/`));
      return suggestClosest(roomQuery, displays.map((d) => d.split("/")[1]), limit)
        .map((r) => `${ukNumOrNull}/${r}`);
    }

    const displays = Array.from(this.roomIndexGlobal.values()).map((v) => v.display);
    return suggestClosest(roomQuery, displays, limit);
  }

  getTeacherItems(teacherDisplay) {
    const key = normText(teacherDisplay);
    return this.teacherIndex.get(key)?.items || [];
  }

  getRoomItems(roomDisplay, ukNumOrNull) {
    if (ukNumOrNull) {
      const key = normText(`${ukNumOrNull}/${roomDisplay}`);
      return this.roomIndexByCampus.get(key)?.items || [];
    }
    const key = normText(roomDisplay);
    return this.roomIndexGlobal.get(key)?.items || [];
  }

  _build(data) {
    this.groups.clear();
    this.teacherIndex.clear();
    this.roomIndexGlobal.clear();
    this.roomIndexByCampus.clear();

    const campusUkSet = new Set();
    const campuses = Array.isArray(data) ? data : [];

    for (const campus of campuses) {
      const campusName = String(campus?.name || "").trim(); // "uk1"
      const ukNumFromCampus = ukNumFromCampusName(campusName);

      if (ukNumFromCampus) campusUkSet.add(ukNumFromCampus);

      for (const inst of campus?.institutes || []) {
        const instName = String(inst?.name || "").trim();
        for (const form of inst?.forms || []) {
          const formName = String(form?.name || "").trim();
          for (const g of form?.groups || []) {
            const groupName = String(g?.name || "").trim();
            if (!groupName) continue;

            const dayMap = new Map();
            for (const day of g?.schedule?.days || []) {
              const wd = DAY_ALIASES.get(normText(day?.name || ""));
              if (wd === undefined) continue;
              dayMap.set(wd, Array.isArray(day?.lessons) ? day.lessons : []);
            }

            const meta = {
              campusName,
              ukNum: ukNumFromCampus,
              institute: instName,
              form: formName
            };

            this.groups.set(groupName, { meta, days: dayMap });

            for (const [weekday, lessons] of dayMap.entries()) {
              for (const l of lessons) {
                const subject = String(l?.subject || "");
                const teacher = parseTeacher(subject);
                const room = parseRoom(subject);

                const ukNumResolved = parseUkNumFromSubject(subject) || ukNumFromCampus || null;
                

                const occ = {
                  group: groupName,
                  weekday,
                  time_from: String(l?.time_from || ""),
                  time_to: String(l?.time_to || ""),
                  week: l?.week || null,
                  subject,
                  teacher,
                  room,
                  ukNum: ukNumResolved,
                  institute: instName,
                  form: formName
                };

                if (teacher) {
                  const tKey = normText(teacher);
                  const bucket = this.teacherIndex.get(tKey) || { display: teacher, items: [] };
                  bucket.items.push(occ);
                  this.teacherIndex.set(tKey, bucket);
                }

                if (room) {
                  const rKey = normText(room);
                  const gBucket = this.roomIndexGlobal.get(rKey) || { display: room, items: [] };
                  gBucket.items.push(occ);
                  this.roomIndexGlobal.set(rKey, gBucket);

                  if (ukNumResolved) {
                    const cKey = normText(`${ukNumResolved}/${room}`);
                    const cDisp = `${ukNumResolved}/${room}`;
                    const cBucket = this.roomIndexByCampus.get(cKey) || { display: cDisp, items: [] };
                    cBucket.items.push(occ);
                    this.roomIndexByCampus.set(cKey, cBucket);
                  }
                }
              }
            }
          }
        }
      }
    }

    this.groupList = Array.from(this.groups.keys());
    this.ukNums = Array.from(campusUkSet).sort((a, b) => Number(a) - Number(b));
  }
}