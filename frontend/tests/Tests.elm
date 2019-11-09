module Tests exposing (..)

import Expect
import Json.Decode as D
import Main exposing (Namespace, Status, msgDecoder, nsDecoder, statusDecoder)
import Test exposing (..)


boldJson =
    """
        {
            "name": "bold-person",
            "hasDownQuota": true,
            "canExtend": true,
            "memUsed": 3,
            "memLimit": 50,
            "autoStartHour": 10,
            "remaining": "6h 14m"
        }
    """


beautifulJson =
    """
        {
            "name": "beautiful-spider",
            "hasDownQuota": false,
            "canExtend": false,
            "memUsed": 45,
            "memLimit": 100
        }
    """


creepyJson =
    """
        {
            "name": "creepy-demon",
            "hasDownQuota": false,
            "canExtend": true,
            "memUsed": 6,
            "memLimit": 20,
            "autoStartHour": 8
        }
    """


jsonList =
    "[" ++ boldJson ++ "," ++ beautifulJson ++ "," ++ creepyJson ++ "]"


jsonMessage =
    "{\"namespaces\":" ++ jsonList ++ ",\"clock\": \"20:14 UTC\"}"


creepy : Namespace
creepy =
    { name = "creepy-demon"
    , hasDownQuota = False
    , canExtend = True
    , memUsed = 6
    , memLimit = 20
    , autoStartHour = Just 8
    , remaining = Nothing
    }


beautiful : Namespace
beautiful =
    { name = "beautiful-spider"
    , hasDownQuota = False
    , canExtend = False
    , memUsed = 45
    , memLimit = 100
    , autoStartHour = Nothing
    , remaining = Nothing
    }


bold : Namespace
bold =
    { name = "bold-person"
    , hasDownQuota = True
    , canExtend = True
    , memUsed = 3
    , memLimit = 50
    , autoStartHour = Just 10
    , remaining = Just "6h 14m"
    }


msg : Test
msg =
    test "Decode full status message" <|
        \_ ->
            Expect.equal
                (msgDecoder jsonMessage)
            <|
                Ok (Status "20:14 UTC" [ bold, beautiful, creepy ])


several : Test
several =
    test "Decode list of namespaces from JSON" <|
        \_ ->
            Expect.equal
                (D.decodeString (D.list nsDecoder) jsonList)
                (Ok [ bold, beautiful, creepy ])


one : Test
one =
    test "Decode namespace from JSON" <|
        \_ ->
            Expect.equal
                (D.decodeString nsDecoder beautifulJson)
                (Ok beautiful)
